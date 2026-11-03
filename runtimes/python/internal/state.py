from dataclasses import dataclass
from typing import Any
import logging
from urllib.parse import urlparse
from nats.aio.client import Client
from nats.js import JetStreamContext
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine
from redis.asyncio import Redis, RedisCluster, ConnectionPool
import nats

from .conf import settings


@dataclass
class AppState:
    nc: Client
    js: JetStreamContext
    engine: AsyncEngine
    redis: ConnectionPool

    @classmethod
    async def new(cls) -> Any:
        nc, js = await get_jetstream()
        engine = await get_db_engine()
        redis = await get_redis_pool()
        return cls(nc, js, engine, redis)

    #def __del__(self) -> None:
    #    loop = asyncio.get_event_loop()
    #    if loop.is_running:
    #        loop.create_task(self.js._nc.close())
    #    else:
    #        loop.run_until_complete(self.js._nc.close())


async def get_jetstream() -> tuple[Client, JetStreamContext]:
    validate_settings()
    logging.info(
        "settings: project=%s nats_url=%s database_url_set=%s redis_url_set=%s",
        settings.project,
        safe_url_summary(settings.nats_url),
        bool(settings.database_url),
        bool(settings.redis_url),
    )
    nc: Client = await nats.connect(settings.nats_url)
    js: JetStreamContext = nc.jetstream()
    try:
        await js.add_stream(
            name=settings.project,
            subjects=[f"{settings.project}.>"],
        )
    except Exception as exc:
        if "stream name already in use" not in str(exc).lower():
            raise RuntimeError(
                f"ERR-NATS-STREAM: name={settings.project} subjects={[f'{settings.project}.>']}: {exc}"
            ) from exc
    return nc, js


async def get_db_engine() -> AsyncEngine:
    return create_async_engine(
        settings.database_url.replace("postgresql", "postgresql+asyncpg"),
        pool_size=settings.db_pool_size,
        max_overflow=settings.db_max_overflow,
        pool_timeout=settings.db_pool_timeout,
        pool_recycle=settings.db_pool_recycle,
        connect_args={"application_name": settings.project},
    )


async def get_redis_pool() -> ConnectionPool:
    redis_password = settings.redis_password or None
    if settings.use_redis_cluster:
        pool = RedisCluster.from_url(
            settings.redis_url,
            password=redis_password,
            max_connections=settings.redis_max_connections,
            decode_responses=True,
        )

        def get_conn(self):
            return RedisCluster(connection_pool=pool)

        setattr(pool, "get_conn", get_conn)
    else:
        pool = ConnectionPool.from_url(
            settings.redis_url,
            password=redis_password,
            max_connections=settings.redis_max_connections,
            decode_responses=True,
        )

        def get_conn(self):
            return Redis(connection_pool=pool)

        setattr(pool, "get_conn", get_conn)
    return pool


def validate_settings() -> None:
    if not settings.project:
        raise ValueError("ERR-SETTINGS: PROJECT is required (set env PROJECT)")
    if not settings.nats_url:
        raise ValueError("ERR-SETTINGS: NATS_URL is required (set env NATS_URL)")
    if not settings.database_url:
        raise ValueError("ERR-SETTINGS: DATABASE_URL is required (set env DATABASE_URL)")
    if not settings.redis_url:
        raise ValueError("ERR-SETTINGS: REDIS_URL is required (set env REDIS_URL)")


def safe_url_summary(raw: str) -> str:
    if not raw:
        return "(empty)"
    try:
        parsed = urlparse(raw)
    except Exception:
        return "(invalid)"
    if not parsed.scheme or not parsed.netloc:
        return "(invalid)"
    host = parsed.netloc.split("@")[-1]
    return f"{parsed.scheme}://{host}"
