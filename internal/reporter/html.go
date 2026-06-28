package reporter

import (
	"html/template"
	"io"
	"strings"

	"github.com/bryanathallah/db-schema-differ/models"
)

type HTMLReporter struct{}

func NewHTMLReporter() Reporter {
	return &HTMLReporter{}
}

func (r *HTMLReporter) Report(diff *models.SchemaDiff, w io.Writer) error {
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
	}
	tmpl, err := template.New("html-report").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, diff)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DB Schema Diff Report</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0d0f12;
            --card-bg: rgba(22, 26, 33, 0.8);
            --border-color: #21262d;
            --text-color: #e6edf3;
            --text-muted: #8d96a0;
            --primary: #58a6ff;
            --success: #3fb950;
            --warning: #d29922;
            --danger: #f85149;
            --safe-bg: rgba(63, 185, 80, 0.1);
            --warn-bg: rgba(210, 153, 34, 0.1);
            --danger-bg: rgba(248, 81, 73, 0.1);
        }

        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: 'Outfit', sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            line-height: 1.6;
            padding: 2rem 1rem;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        header {
            margin-bottom: 2.5rem;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 1.5rem;
            display: flex;
            justify-content: space-between;
            align-items: flex-end;
        }

        h1 {
            font-size: 2.2rem;
            font-weight: 700;
            letter-spacing: -0.5px;
            background: linear-gradient(90deg, #58a6ff, #bc8cff);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .header-meta {
            text-align: right;
            font-size: 0.95rem;
            color: var(--text-muted);
        }

        .header-meta span {
            color: var(--primary);
            font-weight: 600;
        }

        /* Summary Cards */
        .summary-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 1rem;
            margin-bottom: 2.5rem;
        }

        .card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 1.25rem;
            backdrop-filter: blur(10px);
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
            transition: transform 0.2s ease, border-color 0.2s ease;
        }

        .card:hover {
            transform: translateY(-2px);
            border-color: #30363d;
        }

        .card-title {
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 1px;
            color: var(--text-muted);
            margin-bottom: 0.5rem;
            font-weight: 500;
        }

        .card-val {
            font-size: 1.8rem;
            font-weight: 700;
        }

        .card-val.success { color: var(--success); }
        .card-val.warning { color: var(--warning); }
        .card-val.danger { color: var(--danger); }
        .card-val.info { color: var(--primary); }

        .card-desc {
            font-size: 0.8rem;
            color: var(--text-muted);
            margin-top: 0.25rem;
        }

        /* Filter Controls */
        .controls {
            display: flex;
            gap: 0.5rem;
            margin-bottom: 1.5rem;
        }

        .btn {
            background: #21262d;
            border: 1px solid var(--border-color);
            color: var(--text-color);
            padding: 0.5rem 1rem;
            border-radius: 8px;
            cursor: pointer;
            font-family: inherit;
            font-size: 0.9rem;
            font-weight: 500;
            transition: all 0.2s ease;
        }

        .btn:hover {
            background: #30363d;
            border-color: #8d96a0;
        }

        .btn.active {
            background: var(--primary);
            border-color: var(--primary);
            color: #0d0f12;
            font-weight: 600;
        }

        /* Detail List */
        .table-section {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            margin-bottom: 1.5rem;
            overflow: hidden;
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
        }

        .table-header {
            padding: 1.25rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid var(--border-color);
            background: rgba(33, 38, 45, 0.3);
        }

        .table-title {
            font-size: 1.15rem;
            font-weight: 600;
            display: flex;
            align-items: center;
            gap: 0.75rem;
        }

        .table-title .action-indicator {
            width: 8px;
            height: 8px;
            border-radius: 50%;
        }

        .action-add { background-color: var(--success); }
        .action-modify { background-color: var(--warning); }
        .action-drop { background-color: var(--danger); }

        .badge {
            font-size: 0.75rem;
            font-weight: 600;
            padding: 0.2rem 0.5rem;
            border-radius: 6px;
            text-transform: uppercase;
        }

        .badge.safe { background: var(--safe-bg); color: var(--success); border: 1px solid rgba(63, 185, 80, 0.3); }
        .badge.warning { background: var(--warn-bg); color: var(--warning); border: 1px solid rgba(210, 153, 34, 0.3); }
        .badge.danger { background: var(--danger-bg); color: var(--danger); border: 1px solid rgba(248, 81, 73, 0.3); }

        .table-body {
            padding: 1.25rem;
        }

        .sub-section {
            margin-bottom: 1.25rem;
        }

        .sub-section:last-child {
            margin-bottom: 0;
        }

        .sub-title {
            font-size: 0.9rem;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: var(--text-muted);
            margin-bottom: 0.5rem;
            font-weight: 600;
            border-bottom: 1px dashed var(--border-color);
            padding-bottom: 0.25rem;
        }

        .change-item {
            display: flex;
            align-items: flex-start;
            gap: 0.5rem;
            margin-bottom: 0.4rem;
            font-size: 0.95rem;
        }

        .change-item:last-child {
            margin-bottom: 0;
        }

        .symbol {
            font-weight: bold;
            font-family: 'JetBrains Mono', monospace;
        }
        .symbol.add { color: var(--success); }
        .symbol.drop { color: var(--danger); }
        .symbol.modify { color: var(--warning); }

        .change-details {
            display: flex;
            flex-direction: column;
            gap: 0.15rem;
        }

        .change-desc {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.85rem;
            color: var(--text-muted);
            background: #161b22;
            padding: 0.2rem 0.5rem;
            border-radius: 4px;
            border: 1px solid var(--border-color);
        }

        .no-changes {
            text-align: center;
            padding: 3rem;
            background: var(--card-bg);
            border: 1px dashed var(--border-color);
            border-radius: 12px;
            color: var(--text-muted);
        }

        /* Filter visibility classes */
        .hide { display: none !important; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div>
                <h1>DB Schema Diff Report</h1>
                <p style="color: var(--text-muted); margin-top: 0.25rem;">Compare & migrate database schemas cleanly</p>
            </div>
            <div class="header-meta">
                <p>Source DB: <span>{{.Summary.SourceDB}}</span></p>
                <p>Target DB: <span>{{.Summary.TargetDB}}</span></p>
            </div>
        </header>

        <!-- Summary Statistics -->
        <div class="summary-grid">
            <div class="card">
                <div class="card-title">Tables</div>
                <div class="card-val info">{{.Summary.TablesAdded}} / {{.Summary.TablesModified}} / {{.Summary.TablesDropped}}</div>
                <div class="card-desc">Added / Modified / Dropped</div>
            </div>
            <div class="card">
                <div class="card-title">Columns</div>
                <div class="card-val info">{{.Summary.ColumnsAdded}} / {{.Summary.ColumnsModified}} / {{.Summary.ColumnsDropped}}</div>
                <div class="card-desc">Added / Modified / Dropped</div>
            </div>
            <div class="card">
                <div class="card-title">Indexes & FKs</div>
                <div class="card-val info">+{{add .Summary.IndexesAdded .Summary.FKsAdded}} / -{{add .Summary.IndexesDropped .Summary.FKsDropped}}</div>
                <div class="card-desc">Added / Dropped Constraints</div>
            </div>
            <div class="card">
                <div class="card-title">Risk Flags</div>
                <div class="card-val {{if gt .Summary.DangersCount 0}}danger{{else if gt .Summary.WarningsCount 0}}warning{{else}}success{{end}}">
                    {{.Summary.WarningsCount}}W / {{.Summary.DangersCount}}D
                </div>
                <div class="card-desc">Warnings / Dangers Detected</div>
            </div>
        </div>

        <!-- Controls -->
        <div class="controls">
            <button class="btn active" onclick="filterSeverity('ALL')">All Changes</button>
            <button class="btn" onclick="filterSeverity('SAFE')">Safe Penambahan ({{.Summary.TablesAdded}}T / {{.Summary.ColumnsAdded}}C)</button>
            <button class="btn" onclick="filterSeverity('WARNING')">Warnings ({{.Summary.WarningsCount}})</button>
            <button class="btn" onclick="filterSeverity('DANGER')">Dangers / Drops ({{.Summary.DangersCount}})</button>
        </div>

        <!-- Details -->
        {{if not .Tables}}
        <div class="no-changes">
            <h2>🎉 Schemas are identical!</h2>
            <p>No changes detected between the source and target databases.</p>
        </div>
        {{else}}
        <div class="details-list">
            {{range .Tables}}
            <div class="table-section" data-severity="{{.Severity}}">
                <div class="table-header">
                    <div class="table-title">
                        <span class="action-indicator action-{{lower .Action}}"></span>
                        <strong>{{.TableName}}</strong>
                        <span style="font-size: 0.85rem; color: var(--text-muted); font-weight: normal;">
                            {{if eq .Action "ADD"}}Table added{{else if eq .Action "DROP"}}Table dropped{{else}}Table modified{{end}}
                        </span>
                    </div>
                    <span class="badge {{lower .Severity}}">{{.Severity}}</span>
                </div>
                
                {{if or .Columns .Indexes .ForeignKeys}}
                <div class="table-body">
                    <!-- Columns -->
                    {{if .Columns}}
                    <div class="sub-section">
                        <div class="sub-title">Columns</div>
                        {{range .Columns}}
                        <div class="change-item">
                            <span class="symbol {{lower .Action}}">
                                {{if eq .Action "ADD"}}+{{else if eq .Action "DROP"}}-{{else}}~{{end}}
                            </span>
                            <div class="change-content">
                                <strong>{{.ColumnName}}</strong> 
                                <span class="badge {{lower .Severity}}" style="font-size: 0.65rem; padding: 0.05rem 0.3rem;">{{.Severity}}</span>
                                <div class="change-details">
                                    {{range .Changes}}
                                    <span class="change-desc">{{.}}</span>
                                    {{end}}
                                </div>
                            </div>
                        </div>
                        {{end}}
                    </div>
                    {{end}}

                    <!-- Indexes -->
                    {{if .Indexes}}
                    <div class="sub-section">
                        <div class="sub-title">Indexes</div>
                        {{range .Indexes}}
                        <div class="change-item">
                            <span class="symbol {{lower .Action}}">
                                {{if eq .Action "ADD"}}+{{else}}-{{end}}
                            </span>
                            <div>
                                <strong>{{.IndexName}}</strong> 
                                <span style="font-size: 0.85rem; color: var(--text-muted);">
                                    {{if eq .Action "ADD"}}added{{else}}dropped{{end}}
                                </span>
                            </div>
                        </div>
                        {{end}}
                    </div>
                    {{end}}

                    <!-- Foreign Keys -->
                    {{if .ForeignKeys}}
                    <div class="sub-section">
                        <div class="sub-title">Foreign Keys</div>
                        {{range .ForeignKeys}}
                        <div class="change-item">
                            <span class="symbol {{lower .Action}}">
                                {{if eq .Action "ADD"}}+{{else}}-{{end}}
                            </span>
                            <div>
                                <strong>{{.FKName}}</strong>
                                <span style="font-size: 0.85rem; color: var(--text-muted);">
                                    {{if eq .Action "ADD"}}added{{else}}dropped{{end}}
                                </span>
                            </div>
                        </div>
                        {{end}}
                    </div>
                    {{end}}
                </div>
                {{end}}
            </div>
            {{end}}
        </div>
        {{end}}
    </div>

    <script>
        function filterSeverity(severity) {
            // Update button states
            const buttons = document.querySelectorAll('.controls .btn');
            buttons.forEach(btn => btn.classList.remove('active'));
            event.target.classList.add('active');

            // Filter sections
            const sections = document.querySelectorAll('.table-section');
            sections.forEach(sec => {
                if (severity === 'ALL') {
                    sec.classList.remove('hide');
                } else {
                    if (sec.getAttribute('data-severity') === severity) {
                        sec.classList.remove('hide');
                    } else {
                        sec.add('hide'); // wait: classList.add!
                        sec.classList.add('hide');
                    }
                }
            });
        }
    </script>
</body>
</html>`
