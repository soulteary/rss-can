import vm from "node:vm";

// Only protocol messages reach stdout. Requests share a process, not a context.
const encoder = new TextEncoder();
const send = (value) => {
  const bytes = encoder.encode(JSON.stringify(value) + "\n");
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
    send({ id: request.id, result });
  } catch (error) {
    send({ id: request.id, error: String(error.stack ?? error) });
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
