package templates

import "encoding/json"

// incidentEscalation: page the on-call with retries, branch on
// acknowledgement, escalate to a manager and emit synthetic timeline events
// on failure.
var incidentEscalation = Template{
	ID:     "incident-escalation",
	Name:   "Incident escalation with retry",
	Source: SourceBuiltIn,
	Description: "Page the on-call engineer with bounded retries, branch on " +
		"acknowledgement, escalate to a manager on failure and record synthetic " +
		"timeline events for the audit log.",
	Tags: []string{"incident", "retry", "escalation"},
	Graph: json.RawMessage(`{
  "schemaVersion": "1",
  "nodes": [
    {"id": "start-1", "kind": "start", "label": "Start", "position": {"x": 0, "y": 0}},
    {"id": "action-1", "kind": "action", "label": "Page on-call", "position": {"x": 0, "y": 120},
     "config": {"actionType": "mock.pager.page", "params": {"onCall": "$trigger.on_call", "incidentId": "$trigger.incident_id"},
                "retry": {"maxAttempts": 3, "backoffMs": 5000}, "onFailure": "escalate"}},
    {"id": "condition-1", "kind": "condition", "label": "Acknowledged?", "position": {"x": 0, "y": 260},
     "config": {"expression": "$steps.action-1.output.acknowledged == true"}},
    {"id": "action-2", "kind": "action", "label": "Record resolution path", "position": {"x": -220, "y": 400},
     "config": {"actionType": "mock.event.emit", "params": {"event": "incident.acknowledged", "incidentId": "$trigger.incident_id"}}},
    {"id": "action-3", "kind": "action", "label": "Escalate to manager", "position": {"x": 220, "y": 400},
     "config": {"actionType": "mock.pager.escalate", "params": {"manager": "incident-manager", "incidentId": "$trigger.incident_id"},
                "retry": {"maxAttempts": 2, "backoffMs": 10000}, "onFailure": "continue"}},
    {"id": "action-4", "kind": "action", "label": "Notify status page", "position": {"x": 220, "y": 520},
     "config": {"actionType": "mock.notify.statuspage", "params": {"incidentId": "$trigger.incident_id", "state": "escalated"}}},
    {"id": "action-5", "kind": "action", "label": "Record escalation event", "position": {"x": 220, "y": 640},
     "config": {"actionType": "mock.event.emit", "params": {"event": "incident.escalated", "incidentId": "$trigger.incident_id"}}},
    {"id": "end-1", "kind": "end", "label": "End", "position": {"x": 0, "y": 780}}
  ],
  "edges": [
    {"id": "e-start-page", "source": "start-1", "sourceHandle": null, "target": "action-1", "targetHandle": null},
    {"id": "e-page-condition", "source": "action-1", "sourceHandle": null, "target": "condition-1", "targetHandle": null},
    {"id": "e-cond-true", "source": "condition-1", "sourceHandle": "true", "target": "action-2", "targetHandle": null, "label": "true"},
    {"id": "e-cond-false", "source": "condition-1", "sourceHandle": "false", "target": "action-3", "targetHandle": null, "label": "false"},
    {"id": "e-escalate-notify", "source": "action-3", "sourceHandle": null, "target": "action-4", "targetHandle": null},
    {"id": "e-notify-record", "source": "action-4", "sourceHandle": null, "target": "action-5", "targetHandle": null},
    {"id": "e-ack-end", "source": "action-2", "sourceHandle": null, "target": "end-1", "targetHandle": null},
    {"id": "e-record-end", "source": "action-5", "sourceHandle": null, "target": "end-1", "targetHandle": null}
  ]
}`),
}
