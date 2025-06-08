from typing import Any, Callable
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
        f"{settings.project}.{settings.environment}.spawn",
        stream=f"{settings.project}-{settings.environment}"
    )
    async for msgs in consumer.fetch():
        msg: Msg = msgs[0]
        # TODO: refresh code from repo
        func_name = msg.data
        function = importlib.import_module(f"pkg.handlers.{func_name}").handle
        asyncio.create_task(start_function(state, function))
        await msg.ack()


async def consume(state: AppState, consumer: Any, handler: Callable):
    logger.debug("waiting for messages...")
    async for msgs in consumer.fetch():
        msg: Msg = msgs[0]
        logger.info("received event")
        await msg.ack()
        req_id: str | None = msg.subject.split(".")[-1]
        if req_id:
            logger.debug(f"request id: {req_id}")
            res: bytes = await handler(state, req_id)
            logger.debug("handler run complete")
            await state.nc.publish(
                f"{settings.project},{function.__name__}.res.py.{req_id}",
                res
            )

async def start_function(state: AppState, name: str):
    module = importlib.import_module(f"/tmp/{settings.project}/functions/py/{name}.py")
    if module.hasattr("handle"):
        consumer = await state.js.pull_subscribe(
            f"{settings.project}.{name}.exec.py.*",
            stream=settings.project
        )
        async for msgs in consumer.fetch():
            msg: Msg = msgs[0]
            await msg.ack()


class TestRuntime(IsolatedAsyncioTestCase):

    PROJECT = "projone"
    NAME = "one"
    async def test_start_fn():
        state = AppState()
        
