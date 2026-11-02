from fastapi import Request
from pydantic import BaseModel


class Input(BaseModel):
    name: str | None = None


class Output(BaseModel):
    message: str


async def handle(request: Request) -> Output:
    data = await request.json()
    input = Input(**data)
    name = input.name or "stranger"
    return Output(message=f"Hello {name} from Python!")
