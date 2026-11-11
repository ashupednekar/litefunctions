from pydantic import BaseModel


class Input(BaseModel):
    name: str | None = None


class Output(BaseModel):
    message: str
