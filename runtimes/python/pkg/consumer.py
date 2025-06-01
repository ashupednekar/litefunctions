from typing import Callable
import importlib
import asyncio

from nats.aio.msg import Msg
from pkg.state import AppState
from pkg.conf import settings


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


async def start_function(state: AppState, function: Callable):
    consumer = await state.js.pull_subscribe(
        f"{settings.project}.{settings.environment}.exec.{function.__name__}",
        stream=f"{settings.project}-{settings.environment}"
    )
    async for msgs in consumer.fetch():
        msg: Msg = msgs[0]
        await msg.ack()

    
