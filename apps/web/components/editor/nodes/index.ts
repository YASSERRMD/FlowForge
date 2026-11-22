import type { NodeTypes } from "@xyflow/react";

import { PlaceholderNode } from "./placeholder";

// Node type registry. Concrete custom nodes replace the placeholder in
// tasks 05.03–05.06.
export const nodeTypes: NodeTypes = {
  start: PlaceholderNode,
  end: PlaceholderNode,
  action: PlaceholderNode,
  delay: PlaceholderNode,
  condition: PlaceholderNode,
  fork: PlaceholderNode,
  join: PlaceholderNode,
};
