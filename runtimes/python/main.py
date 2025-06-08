import asyncio
import pygit2
import os

from pkg.conf import settings
from pkg.consumer import start_function
from pkg.state import AppState

import logging

logger = logging.getLogger(__name__)
log_format = '%(asctime)s [%(levelname)s] - %(message)s'
date_format = '%Y-%m-%d %H:%M:%S'
logging.basicConfig(level=logging.DEBUG,
                    format=log_format,
                    datefmt=date_format)


async def run():
    state: AppState = await AppState.new()
    if os.path.exists(f"/tmp/{settings.project}"):
        pygit2.Repository(f"/tmp/{settings.project}").remotes["origin"].fetch()
    else:
        pygit2.clone_repository(
            url=f"https://github.com/{settings.git_user}/{settings.project}",
            path=f"/tmp/{settings.project}",
        )
    for func in os.listdir(f"/tmp/{settings.project}/functions/py/"):
        func = func[:-3]
        if "__" in func:
            continue
        logger.info(f"spawning listner for function: {func}")
        asyncio.create_task(start_function(state, func))
        await asyncio.sleep(300)


if __name__ == "__main__":
    asyncio.run(run())
