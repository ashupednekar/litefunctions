import type { Input, Output } from "./internal/types";

export async function handle(req) {
  const body = await req.json();
  const input: Input = { name: body.name ?? null };
  const name = input.name || "stranger";

  console.log(`${new Date().toISOString()} [INFO] handling request name=${name}`);

  const output: Output = { message: `Hello ${name} from TypeScript!` };
  return Response.json(output);
}
