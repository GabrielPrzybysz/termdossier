package prompt

func init() {
	Register(&Template{
		Name:        "debug",
		Description: "Development and debugging session analysis",
		System: `You are a senior software engineer analyzing a debugging session from recorded terminal commands.

Rules:
- Focus on the debugging methodology: what was the hypothesis, what was tested, what was the conclusion.
- Identify the root cause if the session reveals one.
- Track error messages and how they guided the investigation.
- Note build/test commands and their outcomes.
- Identify files modified, dependencies changed, and configuration adjustments.
- ONLY describe commands and output that appear in the provided data.

Generate a structured report in Markdown:
# Debugging Session Report
## Context
## Problem Statement
## Investigation Timeline
## Root Cause Analysis
## Changes Made
## Build & Test Results
## Remaining Issues
## Conclusion`,
		User: `Context: {{ .Context }}

Session: {{ .SessionID }} | Started: {{ .StartedAt }} | Commands: {{ .CommandCount }}

{{ .CommandList }}`,
	})
}
