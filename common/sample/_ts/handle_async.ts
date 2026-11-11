import type { Input, Output } from "./internal/types";

export async function* handle(req) {
  const body = await req.json();
  const input: Input = { name: body.name ?? null };
  const name = input.name || "stranger";

  const output: Output = { message: `Hello ${name} from TypeScript!` };
  yield output;
}
