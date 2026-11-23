package templates

import "encoding/json"

// invoiceApproval: fetch an invoice, branch on an approval threshold, and
// either auto-approve with notification or route to a human reviewer.
// All actions are simulated mock types so the template runs without
// external dependencies.
var invoiceApproval = Template{
	ID:     "invoice-approval",
	Name:   "Invoice approval",
	Source: SourceBuiltIn,
	Description: "Fetch an invoice, auto-approve it under a configurable amount " +
		"threshold, otherwise route it to a human reviewer. All actions are mocked.",
	Tags: []string{"finance", "approval", "condition"},
	Graph: json.RawMessage(`{
  "schemaVersion": "1",
  "nodes": [
    {"id": "start-1", "kind": "start", "label": "Start", "position": {"x": 0, "y": 0}},
    {"id": "action-1", "kind": "action", "label": "Fetch invoice", "position": {"x": 0, "y": 120},
     "config": {"actionType": "mock.invoice.fetch", "params": {"invoiceId": "$trigger.invoice_id"}}},
    {"id": "condition-1", "kind": "condition", "label": "Under threshold?", "position": {"x": 0, "y": 260},
     "config": {"expression": "$steps.action-1.output.amount <= $trigger.approval_threshold"}},
    {"id": "action-2", "kind": "action", "label": "Auto-approve", "position": {"x": -220, "y": 400},
     "config": {"actionType": "mock.invoice.approve", "params": {"invoiceId": "$trigger.invoice_id"}}},
    {"id": "action-3", "kind": "action", "label": "Notify approver", "position": {"x": -220, "y": 520},
     "config": {"actionType": "mock.notify.email", "params": {"to": "finance@example.com", "subject": "Invoice auto-approved"}}},
    {"id": "action-4", "kind": "action", "label": "Request review", "position": {"x": 220, "y": 400},
     "config": {"actionType": "mock.review.request", "params": {"assignee": "senior-accountant", "invoiceId": "$trigger.invoice_id"}}},
    {"id": "action-5", "kind": "action", "label": "Notify requester", "position": {"x": 220, "y": 520},
     "config": {"actionType": "mock.notify.email", "params": {"to": "$trigger.requester_email", "subject": "Invoice needs review"}}},
    {"id": "end-1", "kind": "end", "label": "End", "position": {"x": 0, "y": 680}}
  ],
  "edges": [
    {"id": "e-start-fetch", "source": "start-1", "sourceHandle": null, "target": "action-1", "targetHandle": null},
    {"id": "e-fetch-condition", "source": "action-1", "sourceHandle": null, "target": "condition-1", "targetHandle": null},
    {"id": "e-cond-true", "source": "condition-1", "sourceHandle": "true", "target": "action-2", "targetHandle": null, "label": "true"},
    {"id": "e-cond-false", "source": "condition-1", "sourceHandle": "false", "target": "action-4", "targetHandle": null, "label": "false"},
    {"id": "e-approve-notify", "source": "action-2", "sourceHandle": null, "target": "action-3", "targetHandle": null},
    {"id": "e-review-notify", "source": "action-4", "sourceHandle": null, "target": "action-5", "targetHandle": null},
    {"id": "e-notify-end-a", "source": "action-3", "sourceHandle": null, "target": "end-1", "targetHandle": null},
    {"id": "e-notify-end-b", "source": "action-5", "sourceHandle": null, "target": "end-1", "targetHandle": null}
  ]
}`),
}
