# Gantt chart requirements

Use these requirements for every request to create or update the project Gantt chart in the linked project spreadsheet.

## Source and scope

- Use the current project proposal DOCX as the source of task names, hierarchy, durations, and sequencing. Treat the user's requested section as the scope boundary; do not add later proposal sections unless asked.
- Preserve the proposal's meaning and wording for task labels where practical. Keep section headings distinct from their subtasks.
- When assigning work to an agent, include these requirements in the handoff and specify the source document, spreadsheet, requested section, and any user-provided schedule constraints.

## Schedule

- Use explicit durations in the proposal. Represent overlaps where appropriate; do not sum overlapping work as if it were sequential.
- Do not invent a calendar start date. If none is specified, use relative weeks or days and state the project duration clearly.
- Keep parent phase/project durations consistent with the proposal and show the subordinate work within those periods.
- End each requested section with a completion milestone named `<section title> completed` (or the equivalent requested title) and give that milestone a duration of 0 days.
- Keep the chart's duration, phase ranges, task placement, and milestone position internally consistent. Verify the final sheet values and rendered chart after editing.

## Spreadsheet editing

- Update only the requested section. Preserve unrelated sheet data, formulas, formatting, and other project sections.
- Follow the existing spreadsheet's structure and visual conventions where possible. If there is no established convention, use a readable task hierarchy, duration and relative-time fields, and a week-by-week Gantt display.
- After editing, read back the changed cells and visually inspect the chart to confirm labels, durations, overlaps, and the zero-day completion milestone are visible and correct.
