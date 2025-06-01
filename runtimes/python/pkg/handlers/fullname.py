from pydantic import BaseModel


class NameParts(BaseModel):
    fname: str
    lname: str

class Name(BaseModel):
    name: str

def handle(parts: NameParts):
    return Name(name=f"{parts.fname.title()} {parts.lname.title()}")
