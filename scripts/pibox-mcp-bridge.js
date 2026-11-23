const { createRequire } = require("node:module");

// Resolve the SDK bundled with the PiBox image.
const piboxRequire = createRequire("/opt/pibox/extensions/mcp-bridge/package.json");
const { Client } = piboxRequire("@modelcontextprotocol/sdk/client/index.js");
const { StreamableHTTPClientTransport } = piboxRequire("@modelcontextprotocol/sdk/client/streamableHttp.js");
const { Server } = piboxRequire("@modelcontextprotocol/sdk/server/index.js");
const { StdioServerTransport } = piboxRequire("@modelcontextprotocol/sdk/server/stdio.js");
const { CallToolRequestSchema, ListToolsRequestSchema } = piboxRequire("@modelcontextprotocol/sdk/types.js");

async function run() {
  const endpoint = new URL(process.env.AUTONOMOUS_MCP_URL);
  const authorization = process.env.AUTONOMOUS_MCP_AUTHORIZATION;
  const identity = { name: "autonomous-stack-pibox-bridge", version: "1.0.0" };
  const client = new Client(identity);
  const remote = new StreamableHTTPClientTransport(endpoint, {
    requestInit: { headers: { Authorization: authorization }, redirect: "error" },
  });
  await client.connect(remote);

  const server = new Server(identity, { capabilities: { tools: {} } });
  server.setRequestHandler(ListToolsRequestSchema, ({ params }) => client.listTools(params));
  server.setRequestHandler(CallToolRequestSchema, ({ params }) => client.callTool(params));
  server.onclose = () => {
    client.close().catch(() => process.exitCode = 1);
  };
  await server.connect(new StdioServerTransport());
}

run().catch(() => {
  process.stderr.write("PiBox MCP bridge failed to start. Check the endpoint and token.\n");
  process.exit(1);
});
