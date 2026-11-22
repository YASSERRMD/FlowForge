import type { NodeTypes } from "@xyflow/react";

import { StartNode } from "./start-node";
import { EndNode } from "./end-node";
import { ActionNode, DelayNode } from "./action-node";
import { ConditionNode } from "./condition-node";
import { PlaceholderNode } from "./placeholder";

// Node type registry. Fork/join replace the placeholder in task 05.06.
export const nodeTypes: NodeTypes = {
  start: StartNode,
  end: EndNode,
  action: ActionNode,
  delay: DelayNode,
  condition: ConditionNode,
  fork: PlaceholderNode,
  join: PlaceholderNode,
};
