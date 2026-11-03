export async function handle(req) {
  class Input {
    constructor(obj = {}) {
      this.name = obj.name ?? null;
    }
  }

  class Output {
    constructor(message) {
      this.message = message;
    }
  }

  const body = await req.json();
  const input = new Input(body);
  const name = input.name || "stranger";

  return Response.json(
    new Output(`Hello ${name} from TypeScript!`)
  );
}
