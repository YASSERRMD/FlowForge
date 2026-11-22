import type { NodeTypes } from "@xyflow/react";

import { StartNode } from "./start-node";
import { EndNode } from "./end-node";
import { ActionNode, DelayNode } from "./action-node";
import { ConditionNode } from "./condition-node";
import { ForkNode, JoinNode } from "./fork-join-node";

export const nodeTypes: NodeTypes = {
  start: StartNode,
  end: EndNode,
  action: ActionNode,
  delay: DelayNode,
  condition: ConditionNode,
  fork: ForkNode,
  join: JoinNode,
};
