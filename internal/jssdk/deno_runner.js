import vm from "node:vm";

// Only protocol messages reach stdout. Requests share a process, not a context.
const encoder = new TextEncoder();
const maxMessageBytes = 16 * 1024 * 1024;
const send = (value) => {
  const serialized = JSON.stringify(value);
  if (serialized.length >= maxMessageBytes) throw new Error("JavaScript response exceeds size limit");
  const bytes = encoder.encode(serialized + "\n");
  if (bytes.length > maxMessageBytes) throw new Error("JavaScript response exceeds size limit");
  let offset = 0;
  while (offset < bytes.length) offset += Deno.stdout.writeSync(bytes.subarray(offset));
};
function execute(request) {
  try {
    const context = vm.createContext({});
    const options = { filename: request.filename };
    vm.runInContext(request.base, context, options);
    context.__rssCanResult = vm.runInContext(request.source, context, options);
    const result = vm.runInContext("String(__rssCanResult)", context, options);
    if (result.length > maxMessageBytes) throw new Error("JavaScript result exceeds size limit");
    send({ id: request.id, result });
  } catch (error) {
    send({ id: request.id, error: String(error.stack ?? error).slice(0, 4096) });
  }
}
send({ ready: true });
let input = "";
const decoder = new TextDecoder();
for await (const chunk of Deno.stdin.readable) {
  input += decoder.decode(chunk, { stream: true });
  let newline;
  while ((newline = input.indexOf("\n")) !== -1) {
    const line = input.slice(0, newline);
    input = input.slice(newline + 1);
    execute(JSON.parse(line));
  }
}
