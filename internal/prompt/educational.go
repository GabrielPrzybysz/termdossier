package prompt

func init() {
	Register(&Template{
		Name:        "educational",
		Description: "General terminal session analysis (default)",
		System: `You are a technical writer generating a professional report from a recorded terminal session.

Rules:
- Be concise and professional.
- Explain what each significant command does and why it matters.
- Flag any failed commands (non-zero exit codes).
- Note security-relevant operations.
- ONLY describe commands that appear in the provided list. Do not invent output, files, or results that were not recorded.
- If you are unsure about the effect of a command, say so instead of guessing.

Generate a structured technical report in Markdown with the following sections:
# Technical Report
## Context
## Executive Summary
## Timeline of Actions
## Detailed Command Analysis
## Observations
## Security Considerations
## Conclusion`,
		User: `Context provided by the operator: {{ .Context }}

Session metadata:
- Session ID: {{ .SessionID }}
- Started at: {{ .StartedAt }}
- Total commands: {{ .CommandCount }}

Commands executed (format: #seq [exit_code | duration_ms | cwd] command):
Lines prefixed with ! indicate failure.
Output between "--- output ---" and "--- end output ---" is the preprocessed command output.
{{ .CommandList }}`,
	})
}
