from dataclasses import dataclass
from nats.aio.client import Client
from nats.js import JetStreamContext
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine
from redis.asyncio import Redis, RedisCluster, ConnectionPool, ClusterConnectionPool
import asyncio
import nats

from .conf import settings


@dataclass
class AppState:
    prefix: str
    nc: Client
    js: JetStreamContext
    engine: AsyncEngine
    redis: ConnectionPool

    def __init__(self) -> None:
        self.nc, self.js = get_jetstream()
        self.engine = get_db_engine()
        self.redis = get_redis_pool()

    def __del__(self) -> None:
        loop = asyncio.get_event_loop()
        if loop.is_running:
            loop.create_task(self.js._nc.close())
        else:
            loop.run_until_complete(self.js._nc.close())


async def get_jetstream() -> tuple[Client, JetStreamContext]:
    nc: Client = await nats.connect(settings.nats_broker_url)
    js: JetStreamContext = nc.jetstream()
    js.add_stream(
        name=settings.project,
        subjects=[f"{settings.project}.>"],
    )
    return nc, js


async def get_db_engine() -> AsyncEngine:
    return create_async_engine(
        settings.database_url,
        pool_size=settings.db_pool_size,
        max_overflow=settings.db_max_overflow,
        pool_timeout=settings.db_pool_timeout,
        pool_recycle=settings.db_pool_recycle,
        connect_args={"application_name": f"{settings.project}-{settings.environment}"},
    )


async def get_redis_pool() -> ConnectionPool:
    if settings.use_redis_cluster:
        pool = ClusterConnectionPool.from_url(
            settings.redis_url,
            max_connections=settings.redis_max_connections,
            decode_responses=True,
        )

        def get_conn(self):
            return RedisCluster(connection_pool=pool)

        setattr(pool, "get_conn", get_conn)
    else:
        pool = ConnectionPool.from_url(
            settings.redis_url,
            max_connections=settings.redis_max_connections,
            decode_responses=True,
        )

        def get_conn(self):
            return Redis(connection_pool=pool)

        setattr(pool, "get_conn", get_conn)
    return pool
