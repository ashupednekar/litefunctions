// Small progressive-enhancement helpers for the server-rendered invoice UI.
// Every action also works as a plain HTML form when JS is unavailable.

(function () {
  "use strict";

  // Make whole table rows clickable without breaking links and buttons.
  document.addEventListener("click", function (event) {
    const row = event.target.closest("[data-href]");
    if (!row) return;
    if (event.target.closest("a, button, input, select, textarea, form, label")) return;
    window.location.href = row.dataset.href;
  });

  // Line item editor: add and remove rows.
  const container = document.querySelector("[data-line-items]");
  const template = document.getElementById("line-row-template");
  const addButton = document.querySelector("[data-add-line]");

  function bindRemove(row) {
    const remove = row.querySelector("[data-remove-line]");
    if (!remove) return;
    remove.addEventListener("click", function () {
      const rows = container.querySelectorAll(".line-row");
      if (rows.length === 1) {
        rows[0].querySelectorAll("input").forEach(function (input) {
          input.value = input.name === "line_quantity" ? "1" : "";
        });
        return;
      }
      row.remove();
    });
  }

  if (container && template && addButton) {
    container.querySelectorAll(".line-row").forEach(bindRemove);
    addButton.addEventListener("click", function () {
      const fragment = template.content.cloneNode(true);
      const row = fragment.querySelector(".line-row");
      container.appendChild(fragment);
      bindRemove(row);
      const description = row.querySelector('input[name="line_description"]');
      if (description) description.focus();
    });
  }

  // Default the due date from the selected customer's payment terms.
  const customer = document.querySelector('select[name="customer_id"]');
  const issueDate = document.querySelector('input[name="issue_date"]');
  const dueDate = document.querySelector('input[name="due_date"]');
  if (customer && issueDate && dueDate) {
    const syncDueDate = function () {
      const option = customer.options[customer.selectedIndex];
      const terms = option ? parseInt(option.dataset.terms || "", 10) : NaN;
      if (!issueDate.value || isNaN(terms)) return;
      const date = new Date(issueDate.value + "T00:00:00");
      if (isNaN(date.getTime())) return;
      date.setDate(date.getDate() + terms);
      dueDate.value = date.toISOString().slice(0, 10);
    };
    customer.addEventListener("change", syncDueDate);
  }
})();
