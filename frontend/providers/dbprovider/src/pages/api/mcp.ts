import { createMcpApiHandler } from 'sealos-mcp-sdk';
import path from 'path';

const handler = createMcpApiHandler(
  path.join(process.cwd(), 'public', 'database.json'),
  'https://dbprovider.gzg.sealos.run'
);
export default handler;
