import type { NodeTypes } from "@xyflow/react";

import { StartNode } from "./start-node";
import { EndNode } from "./end-node";
import { ActionNode, DelayNode } from "./action-node";
import { PlaceholderNode } from "./placeholder";

// Node type registry. Remaining custom nodes replace the placeholder in
// tasks 05.05–05.06.
export const nodeTypes: NodeTypes = {
  start: StartNode,
  end: EndNode,
  action: ActionNode,
  delay: DelayNode,
  condition: PlaceholderNode,
  fork: PlaceholderNode,
  join: PlaceholderNode,
};
