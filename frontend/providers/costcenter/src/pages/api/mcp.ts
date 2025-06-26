import { createMcpApiHandler } from 'sealos-mcp-sdk';
import path from 'path';

export const dynamic = 'force-dynamic';
const handler = createMcpApiHandler(
  path.join(process.cwd(), 'public', 'costcenter.json'),
  'https://costcenter.gzg.sealos.run'
);
export default handler;
