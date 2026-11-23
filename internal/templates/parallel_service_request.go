package templates

import "encoding/json"

// parallelServiceRequest: a service request fans out into three simulated
// validation checks running in parallel, joins, and branches on the
// combined result.
var parallelServiceRequest = Template{
	ID:     "parallel-service-request",
	Name:   "Parallel service request validation",
	Source: SourceBuiltIn,
	Description: "Validate a service request with three parallel simulated " +
		"checks (inventory, credit, address), join the results and branch on the outcome.",
	Tags: []string{"operations", "parallel", "fork-join"},
	Graph: json.RawMessage(`{
  "schemaVersion": "1",
  "nodes": [
    {"id": "start-1", "kind": "start", "label": "Start", "position": {"x": 0, "y": 0}},
    {"id": "action-1", "kind": "action", "label": "Load request", "position": {"x": 0, "y": 110},
     "config": {"actionType": "mock.request.load", "params": {"requestId": "$trigger.request_id"}}},
    {"id": "fork-1", "kind": "fork", "label": "Run checks", "position": {"x": 0, "y": 230},
     "config": {"branchCount": 3}},
    {"id": "action-2", "kind": "action", "label": "Check inventory", "position": {"x": -320, "y": 360},
     "config": {"actionType": "mock.check.inventory", "params": {"sku": "$trigger.sku"}}},
    {"id": "action-3", "kind": "action", "label": "Check credit", "position": {"x": 0, "y": 360},
     "config": {"actionType": "mock.check.credit", "params": {"customerId": "$trigger.customer_id"}}},
    {"id": "action-4", "kind": "action", "label": "Check address", "position": {"x": 320, "y": 360},
     "config": {"actionType": "mock.check.address", "params": {"addressId": "$trigger.address_id"}}},
    {"id": "join-1", "kind": "join", "label": "Collect results", "position": {"x": 0, "y": 500},
     "config": {"branchCount": 3}},
    {"id": "condition-1", "kind": "condition", "label": "All checks passed?", "position": {"x": 0, "y": 620},
     "config": {"expression": "$steps.action-2.output.ok and $steps.action-3.output.ok and $steps.action-4.output.ok"}},
    {"id": "action-5", "kind": "action", "label": "Fulfill request", "position": {"x": -180, "y": 750},
     "config": {"actionType": "mock.request.fulfill", "params": {"requestId": "$trigger.request_id"}}},
    {"id": "action-6", "kind": "action", "label": "Reject with reasons", "position": {"x": 180, "y": 750},
     "config": {"actionType": "mock.request.reject", "params": {"requestId": "$trigger.request_id"}}},
    {"id": "end-1", "kind": "end", "label": "End", "position": {"x": 0, "y": 880}}
  ],
  "edges": [
    {"id": "e-start-load", "source": "start-1", "sourceHandle": null, "target": "action-1", "targetHandle": null},
    {"id": "e-load-fork", "source": "action-1", "sourceHandle": null, "target": "fork-1", "targetHandle": null},
    {"id": "e-fork-inventory", "source": "fork-1", "sourceHandle": "branch-1", "target": "action-2", "targetHandle": null, "label": "branch-1"},
    {"id": "e-fork-credit", "source": "fork-1", "sourceHandle": "branch-2", "target": "action-3", "targetHandle": null, "label": "branch-2"},
    {"id": "e-fork-address", "source": "fork-1", "sourceHandle": "branch-3", "target": "action-4", "targetHandle": null, "label": "branch-3"},
    {"id": "e-inventory-join", "source": "action-2", "sourceHandle": null, "target": "join-1", "targetHandle": null},
    {"id": "e-credit-join", "source": "action-3", "sourceHandle": null, "target": "join-1", "targetHandle": null},
    {"id": "e-address-join", "source": "action-4", "sourceHandle": null, "target": "join-1", "targetHandle": null},
    {"id": "e-join-condition", "source": "join-1", "sourceHandle": null, "target": "condition-1", "targetHandle": null},
    {"id": "e-cond-true", "source": "condition-1", "sourceHandle": "true", "target": "action-5", "targetHandle": null, "label": "true"},
    {"id": "e-cond-false", "source": "condition-1", "sourceHandle": "false", "target": "action-6", "targetHandle": null, "label": "false"},
    {"id": "e-fulfill-end", "source": "action-5", "sourceHandle": null, "target": "end-1", "targetHandle": null},
    {"id": "e-reject-end", "source": "action-6", "sourceHandle": null, "target": "end-1", "targetHandle": null}
  ]
}`),
}
