from fastapi import Request

from .internal.types import Input, Output


async def handle(request: Request) -> Output:
    data = await request.json()
    input = Input(**data)
    name = input.name or "stranger"
    return Output(message=f"Hello {name} from Python!")
