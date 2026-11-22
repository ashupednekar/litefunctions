"""Exact money handling.

Money is stored as integer minor units (cents, pence, yen, fils...). The API
speaks decimal strings in major units so JSON floats never enter the domain.
"""

from __future__ import annotations

from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from typing import Any

from .errors import ValidationError

# ISO 4217 currencies with a non-default exponent. Everything else is 2.
EXPONENTS: dict[str, int] = {
    "BIF": 0,
    "CLP": 0,
    "DJF": 0,
    "GNF": 0,
    "ISK": 0,
    "JPY": 0,
    "KMF": 0,
    "KRW": 0,
    "PYG": 0,
    "RWF": 0,
    "UGX": 0,
    "UYI": 0,
    "VND": 0,
    "VUV": 0,
    "XAF": 0,
    "XOF": 0,
    "XPF": 0,
    "BHD": 3,
    "IQD": 3,
    "JOD": 3,
    "KWD": 3,
    "LYD": 3,
    "OMR": 3,
    "TND": 3,
}


def normalize_currency(value: Any, *, field: str = "currency") -> str:
    if not isinstance(value, str):
        raise ValidationError(f"{field} must be a 3-letter currency code")
    code = value.strip().upper()
    if len(code) != 3 or not code.isalpha():
        raise ValidationError(
            f"{field} must be a 3-letter currency code",
            details={"value": value},
        )
    return code


def exponent_for(currency: str) -> int:
    return EXPONENTS.get(currency.upper(), 2)


def as_decimal(value: Any, *, field: str) -> Decimal:
    """Parse an API number without ever going through binary float math."""
    if isinstance(value, Decimal):
        parsed = value
    elif isinstance(value, bool):
        raise ValidationError(f"{field} must be a number")
    elif isinstance(value, int):
        parsed = Decimal(value)
    elif isinstance(value, float):
        parsed = Decimal(str(value))
    elif isinstance(value, str):
        text = value.strip()
        if not text:
            raise ValidationError(f"{field} must not be empty")
        try:
            parsed = Decimal(text)
        except InvalidOperation as exc:
            raise ValidationError(
                f"{field} must be a decimal number", details={"value": value}
            ) from exc
    else:
        raise ValidationError(
            f"{field} must be a number or decimal string",
            details={"type": type(value).__name__},
        )

    if not parsed.is_finite():
        raise ValidationError(f"{field} must be finite")
    return parsed


def to_minor(amount: Decimal, currency: str) -> int:
    exponent = exponent_for(currency)
    quantum = Decimal(1).scaleb(-exponent)
    return int(amount.quantize(quantum, rounding=ROUND_HALF_UP).scaleb(exponent))


def from_minor(minor: int, currency: str) -> Decimal:
    exponent = exponent_for(currency)
    return Decimal(minor).scaleb(-exponent)


def format_money(minor: int, currency: str) -> str:
    exponent = exponent_for(currency)
    value = from_minor(minor, currency)
    return f"{value:.{exponent}f}"


def parse_money(value: Any, currency: str, *, field: str) -> int:
    return to_minor(as_decimal(value, field=field), currency)


def round_bp(minor: int, basis_points: int) -> int:
    """Apply a basis-point rate (825 = 8.25%) to a minor-unit amount."""
    amount = Decimal(minor) * Decimal(basis_points) / Decimal(10_000)
    return int(amount.quantize(Decimal(1), rounding=ROUND_HALF_UP))


def as_int(value: Any, *, field: str, minimum: int, maximum: int) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        if isinstance(value, str) and value.strip().lstrip("-").isdigit():
            value = int(value.strip())
        else:
            raise ValidationError(f"{field} must be an integer")
    if value < minimum or value > maximum:
        raise ValidationError(
            f"{field} must be between {minimum} and {maximum}",
            details={"value": value},
        )
    return value
