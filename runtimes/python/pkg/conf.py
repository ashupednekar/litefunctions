from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    project: str

    nats_url: str

    git_user: str
    git_token: str

    database_url: str
    db_pool_size: int
    db_max_overflow: str
    db_pool_timeout: str
    db_pool_recycle: str
    db_pool_pre_ping: bool = True

    use_redis_cluster: bool
    redis_url: str
    redis_password: str | None = None
    redis_max_connections: int


settings = Settings()
