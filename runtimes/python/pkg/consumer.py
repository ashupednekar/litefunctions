import sys
from typing import Any, Callable, List
import importlib
import asyncio

from nats.aio.msg import Msg
from unittest.async_case import IsolatedAsyncioTestCase
from pkg.state import AppState
from pkg.conf import settings

import logging

logger = logging.getLogger(__name__)


async def start_internal(state: AppState):
    consumer = await state.js.pull_subscribe(
        f"{settings.project}.spawn", stream=settings.project
    )
    async for msgs in consumer.fetch():
        msg: Msg = msgs[0]
        # TODO: refresh code from repo
        asyncio.create_task(start_function(state, msg.data))
        await msg.ack()


async def consume(state: AppState, consumer: Any, handler: Callable):
    logger.debug("waiting for messages...")
    while True:
        msgs: List[Msg] = await consumer.fetch(timeout=None)
        msg: Msg = msgs[0]
        logger.info("received event")
        await msg.ack()
        req_id: str | None = msg.subject.split(".")[-1]
        if req_id:
            logger.debug(f"request id: {req_id}")
            res: bytes = await handler(state, req_id)
            logger.debug("handler run complete")
            await state.nc.publish(
                f"{settings.project}.{handler.__name__}.res.py.{req_id}", res
            )


async def start_function(state: AppState, name: str):
    spec = importlib.util.spec_from_file_location(
        name, f"/tmp/{settings.project}/functions/py/{name}.py"
    )
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    handler = module.handle
    setattr(handler, "__name__", name)
    if hasattr(module, "handle"):
        consumer = await state.js.pull_subscribe(
            f"{settings.project}.{name}.exec.py.*", stream=settings.project
        )
        await consume(state, consumer, module.handle)


class TestRuntime(IsolatedAsyncioTestCase):
    PROJECT = "projone"
    NAME = "one"

    async def test_start_fn():
        state = AppState()
