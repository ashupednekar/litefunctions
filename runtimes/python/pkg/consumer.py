from typing import Callable

from nats.aio.msg import Msg
from pkg.state import AppState
from pkg.conf import settings


async def start(state: AppState, function: Callable):
    consumer = await state.js.pull_subscribe(
        f"{settings.project}.{settings.environment}.exec.{function.__name__}",
        stream=f"{settings.project}-{settings.environment}"
    )
    async for msgs in consumer.fetch():
        msg: Msg = msgs[0]
        await msg.ack()
    
