import logging

from fastapi import Request

from .internal.types import Input, Output

logger = logging.getLogger(__name__)


async def handle(request: Request):
    data = await request.json()
    input = Input(**data)
    name = input.name or "stranger"
    logger.info("handling request name=%s", name)
    yield Output(message=f"Hello {name} from Python!")
