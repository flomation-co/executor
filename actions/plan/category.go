// Package plan groups the executor's plan-management actions —
// AI-callable tools that let an agent author a multi-step plan and
// (in later milestones) revise, cancel, or query progress on one.
// See plans/agent_planning.md for the design and
// plans/agent_planning_m1.md for the M1 surface (this category
// ships with `create` only).
//
// Why a top-level category rather than sitting under "agent"?
// Plans are a first-class primitive — they outlive any single agent
// turn and have their own lifecycle (draft → active → completed |
// blocked | cancelled). Surfacing them as their own category in
// the editor's action picker makes that prominence explicit.
package plan

const (
	CategoryName        = "Plan"
	CategoryIcon        = "list-check"
	CategoryDescription = "Create and manage autonomous multi-step plans the agent progresses on its own."
)
