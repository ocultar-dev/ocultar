package handlers

import (
	"fmt"

	"github.com/ocultar-dev/ocultar/pkg/audit"
)

// --- Risk Report Generator templates/types, used by the pilot handlers ---

const reportVersion = "3.1"
const engineVersion = "v1.14"

type reportMeta struct {
	ReportID           string
	GeneratedAt        string
	DatasetScope       string
	MethodologyVersion string
	EngineVersion      string
	TotalRecords       int
}

type fullReport struct {
	Meta   reportMeta
	Risk   audit.RiskReport
	Before scenarioSummary
	After  scenarioSummary
}

type scenarioSummary struct {
	Label       string
	RiskLevel   string
	RiskScore   string
	VaRRange    string
	AIStatus    string
	Description string
}

func buildScenarios(r audit.RiskReport) (scenarioSummary, scenarioSummary) {
	before := scenarioSummary{
		Label:       "Scenario A — Current State (No Protection)",
		RiskLevel:   r.OverallRiskLevel,
		RiskScore:   fmt.Sprintf("%.1f / 10", r.OverallRiskScore),
		VaRRange:    fmt.Sprintf("€%.0f – €%.0f (estimated)", r.Exposure.VaRMin, r.Exposure.VaRMax),
		AIStatus:    r.AI.Status,
		Description: "The raw dataset as-is, transmitted directly to an LLM API or stored in a vector database. All PII fields are exposed in plaintext.",
	}

	afterScoreMin := r.OverallRiskScore * 0.05
	afterScoreMax := r.OverallRiskScore * 0.15
	afterVaRMin := r.Exposure.VaRMin * 0.02
	afterVaRMax := r.Exposure.VaRMin * 0.08

	after := scenarioSummary{
		Label:       "Scenario B — After OCULTAR Processing",
		RiskLevel:   "LOW",
		RiskScore:   fmt.Sprintf("%.1f – %.1f / 10 (projected)", afterScoreMin, afterScoreMax),
		VaRRange:    fmt.Sprintf("€%.0f – €%.0f (projected residual)", afterVaRMin, afterVaRMax),
		AIStatus:    "ALLOW",
		Description: "After OCULTAR tokenization and format-preserving encryption pipeline. Direct identifiers are removed and re-identification risk is significantly reduced (though not mathematically eliminated).",
	}
	return before, after
}

const mdTemplate = `# OCULTAR Data Risk Assessment Report

> **CONFIDENTIAL — For Authorised Recipients Only**
> This report constitutes a technical risk and privacy assessment based on automated analysis. It is informational in nature and does not constitute legal advice or a regulatory compliance determination. Distribution is restricted to named stakeholders.

---

## Report Metadata

| Field | Value |
| :--- | :--- |
| **Report ID** | OCU-{{.Meta.ReportID}} |
| **Generated** | {{.Meta.GeneratedAt}} |
| **Dataset Scope** | ` + "`" + `{{.Meta.DatasetScope}}` + "`" + ` |
| **Records Analysed** | {{.Meta.TotalRecords}} |
| **Methodology Version** | v{{.Meta.MethodologyVersion}} |
| **Engine** | Ocultar {{.Meta.EngineVersion}} |

---

## Executive Risk Summary

{{if eq .Risk.OverallRiskLevel "CRITICAL"}}> [!CAUTION]
> **Overall Risk Level: {{.Risk.OverallRiskLevel}} ({{printf "%.1f" .Risk.OverallRiskScore}}/10)**
> **Compliance Likelihood: {{if .Risk.IsGDPRPseudonymized}}✅ Meets Common Pseudonymization Thresholds{{else}}⚠️ High Non-Compliance Likelihood (External Processing Scenarios){{end}}**{{end}}
{{if eq .Risk.OverallRiskLevel "HIGH"}}> [!WARNING]
> **Overall Risk Level: {{.Risk.OverallRiskLevel}} ({{printf "%.1f" .Risk.OverallRiskScore}}/10)**
> **Compliance Likelihood: {{if .Risk.IsGDPRPseudonymized}}✅ Meets Common Pseudonymization Thresholds{{else}}⚠️ High Non-Compliance Likelihood (External Processing Scenarios){{end}}**{{end}}
{{if eq .Risk.OverallRiskLevel "MEDIUM"}}> [!IMPORTANT]
> **Overall Risk Level: {{.Risk.OverallRiskLevel}} ({{printf "%.1f" .Risk.OverallRiskScore}}/10)**
> **Compliance Likelihood: {{if .Risk.IsGDPRPseudonymized}}✅ Meets Common Pseudonymization Thresholds{{else}}⚠️ Elevated Risk — Review Recommended{{end}}**{{end}}
{{if eq .Risk.OverallRiskLevel "LOW"}}> [!NOTE]
> **Overall Risk Level: {{.Risk.OverallRiskLevel}} ({{printf "%.1f" .Risk.OverallRiskScore}}/10)**
> **Compliance Likelihood: ✅ Meets Common Pseudonymization Thresholds**{{end}}

The dataset identified in this report contains an estimated **{{.Risk.ViolatingRecords}} records** that fall below commonly cited EU pseudonymization thresholds. In its current state, this data **{{if .Risk.IsGDPRPseudonymized}}appears to satisfy commonly cited thresholds for use{{else}}presents elevated technical risk for use{{end}} with external AI systems and LLM APIs** without prior sanitisation.

The estimated financial exposure associated with unauthorised disclosure of this dataset is in the range of **€{{printf "%.0f" .Risk.Exposure.VaRMin}} – €{{printf "%.0f" .Risk.Exposure.VaRMax}}**. This is a **simulated estimate** grounded in the OCULTAR Three-Pillar VaR model, incorporating regulatory simulation anchors, operational incident benchmarks, and a risk multiplier. Actual impact is subject to contextual factors and organisational mitigating controls.

---

## Risk Scorecard

| Category | Score | Level | Business Implication |
| :--- | :---: | :---: | :--- |
| **Identifiability Risk** | {{printf "%.1f" .Risk.Identifiability.Score}}/10 | {{.Risk.Identifiability.Label}} | {{.Risk.Identifiability.Implication}} |
| **Financial Sensitivity** | {{printf "%.1f" .Risk.FinancialSensitivity.Score}}/10 | {{.Risk.FinancialSensitivity.Label}} | {{.Risk.FinancialSensitivity.Implication}} |
| **Re-identification Risk** | {{printf "%.1f" .Risk.ReidentificationRisk.Score}}/10 | {{.Risk.ReidentificationRisk.Label}} | {{.Risk.ReidentificationRisk.Implication}} |
| **Compliance Readiness** | {{printf "%.1f" .Risk.ComplianceReadiness.Score}}/10 | {{.Risk.ComplianceReadiness.Label}} | {{.Risk.ComplianceReadiness.Implication}} |
| **Overall** | **{{printf "%.1f" .Risk.OverallRiskScore}}/10** | **{{.Risk.OverallRiskLevel}}** | Weighted composite score (Identifiability 35%, Financial 25%, Re-id 25%, Compliance 15%) |

---

## Technical Metrics — Interpreted

### K-Anonymity
**Raw Score:** {{.Risk.KAnonymity}}

{{.Risk.KAnonymityInterpretation}}

> **Industry Benchmark:** Common industry frameworks suggest a minimum K-score of 3–5 for basic pseudonymization. This is a technical benchmark, not a mandatory legal threshold—contextual factors, processing purpose, and applicable exemptions determine actual compliance obligations.

### L-Diversity
**Raw Score:** {{.Risk.LDiversity}}

{{.Risk.LDiversityInterpretation}}

> **Industry Benchmark:** An L-Diversity score of ≥2 is commonly recommended to mitigate homogeneity attacks, as referenced in ISO/IEC 29101 (Privacy Architecture Framework). This is an industry guideline; applicable legal thresholds depend on jurisdictional context.

---

## Financial Exposure Model

The **Value at Risk (VaR)** range below is computed using a three-component methodology anchored to industry breach cost benchmarks. All figures are **simulated estimates** and should not be interpreted as predicted fine amounts or contractual commitments.

### VaR Components

| Component | Methodology | Min Estimate | Max Estimate |
| :--- | :--- | ---: | ---: |
| **Regulatory Exposure** | Simulation anchor (€10k–€100k base) × Dataset Risk Score ({{printf "%.2f" .Risk.DatasetRiskScore}}) | **€{{printf "%.0f" .Risk.Exposure.RegulatoryExposureMin}}** | **€{{printf "%.0f" .Risk.Exposure.RegulatoryExposureMax}}** |
| **Operational Cost** | Industry benchmark (€100–€300/record) × {{.Risk.TotalRecords}} records | **€{{printf "%.0f" .Risk.Exposure.OperationalCostMin}}** | **€{{printf "%.0f" .Risk.Exposure.OperationalCostMax}}** |
| **Risk Multiplier** | Profile-driven tiering (K={{.Risk.KAnonymity}}, L={{.Risk.LDiversity}}) | **{{printf "%.1f" .Risk.Exposure.RiskMultiplierMin}}×** | **{{printf "%.1f" .Risk.Exposure.RiskMultiplierMax}}×** |
| | | | |
| **Value at Risk (Estimated)** | **(Regulatory + Operational) × Multiplier** | **€{{printf "%.0f" .Risk.Exposure.VaRMin}}** | **€{{printf "%.0f" .Risk.Exposure.VaRMax}}** |

> **Assumptions & Methodology Note:**
> {{.Risk.Exposure.AssumptionsNote}}

---

## AI & LLM Exposure Assessment

### Decision: {{.Risk.AI.Status}}

| Parameter | Assessment |
| :--- | :--- |
| **External LLM API Safety** | {{.Risk.AI.LLMExposure}} risk |
| **Internal Copilot Safety** | {{if eq .Risk.AI.Status "ALLOW"}}✅ Permitted with monitoring{{else if eq .Risk.AI.Status "SANITIZE_FIRST"}}⚠️ Permitted after OCULTAR processing{{else}}🚫 Not recommended without sanitisation{{end}} |
| **Vector DB / RAG Indexing** | {{if .Risk.AI.RAGSafe}}✅ Estimated safe for indexing{{else}}🚫 Not recommended without prior processing{{end}} |

**RAG & Vector Database Guidance:**
{{.Risk.AI.RAGGuidance}}

**Recommended Action:**
{{.Risk.AI.Recommendation}}

---

## Before / After Simulation

This section demonstrates the modelled impact of the Ocultar pipeline on your dataset's risk profile. Figures are projected estimates based on typical processing outcomes.

| Metric | {{.Before.Label}} | {{.After.Label}} |
| :--- | :--- | :--- |
| **Risk Level** | 🔴 {{.Before.RiskLevel}} | 🟢 {{.After.RiskLevel}} |
| **Risk Score** | {{.Before.RiskScore}} | {{.After.RiskScore}} |
| **Financial Exposure (VaR)** | {{.Before.VaRRange}} | {{.After.VaRRange}} |
| **AI / LLM Status** | {{.Before.AIStatus}} | {{.After.AIStatus}} |

**What changes:**
- **Before:** {{.Before.Description}}
- **After:** {{.After.Description}}

---

## Assumptions

The following assumptions underpin all quantitative estimates in this report:

| Assumption | Value / Range | Basis |
| :--- | :--- | :--- |
| **Regulatory anchor (low)** | €10,000 | Simulation baseline |
| **Regulatory anchor (high)** | €100,000 | Simulation ceiling |
| **Operational cost per record** | €100–€300 | Industry study range |
| **Pseudonymization threshold** | K≥3, L≥2 | Common benchmark |

---

## Remediation Plan

{{.Risk.Recommendation}}

---

## Appendix: Methodology & Standards

This report applies the following analytical frameworks:

- **K-Anonymity** (Sweeney, 2002)
- **L-Diversity** (Machanavajjhala et al., 2006)
- **GDPR Article 5(1)(f)**
- **ISO/IEC 29101**

> This report was generated automatically by Ocultar {{.Meta.EngineVersion}}. Technical assessment only.

---

*Ocultar {{.Meta.EngineVersion}} | Methodology v{{.Meta.MethodologyVersion}} | Report ID: OCU-{{.Meta.ReportID}}*
*Generated: {{.Meta.GeneratedAt}}*
`

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>OCULTAR Risk Assessment — OCU-{{.Meta.ReportID}}</title>
<style>
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :root {
    --critical: #dc2626; --high: #ea580c; --medium: #d97706; --low: #16a34a;
    --bg: #f8fafc; --surface: #ffffff; --border: #e2e8f0;
    --text: #0f172a; --muted: #64748b; --accent: #1e40af;
  }
  body { font-family: 'Inter', sans-serif; background: var(--bg); color: var(--text); font-size: 14px; line-height: 1.6; padding: 40px 24px; }
  .container { max-width: 960px; margin: 0 auto; }
  .report-header { background: var(--text); color: white; padding: 40px; border-radius: 12px; margin-bottom: 32px; position: relative; overflow: hidden; }
  .report-header h1 { font-size: 22px; font-weight: 700; margin-bottom: 4px; }
  .meta-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; margin-top: 24px; }
  .meta-item label { display: block; font-size: 10px; text-transform: uppercase; opacity: 0.5; }
  .meta-item span { font-size: 13px; font-weight: 500; }
  .risk-banner { border-radius: 10px; padding: 24px 28px; margin-bottom: 28px; display: flex; align-items: center; gap: 20px; border: 1px solid var(--border); }
  .risk-banner.CRITICAL { background: #fef2f2; border-color: #fecaca; }
  .risk-banner.HIGH { background: #fff7ed; border-color: #fed7aa; }
  .risk-banner.MEDIUM { background: #fffbeb; border-color: #fde68a; }
  .risk-banner.LOW { background: #f0fdf4; border-color: #bbf7d0; }
  .risk-dial { width: 64px; height: 64px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 18px; font-weight: 700; color: white; flex-shrink: 0; }
  .CRITICAL .risk-dial { background: var(--critical); }
  .HIGH .risk-dial { background: var(--high); }
  .MEDIUM .risk-dial { background: var(--medium); }
  .LOW .risk-dial { background: var(--low); }
  .section { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; padding: 28px; margin-bottom: 20px; }
  .section h2 { font-size: 14px; font-weight: 700; margin-bottom: 16px; padding-bottom: 10px; border-bottom: 1px solid var(--border); color: var(--accent); text-transform: uppercase; }
  table { width: 100%; border-collapse: collapse; }
  th { text-align: left; padding: 10px; background: var(--bg); font-size: 11px; text-transform: uppercase; color: var(--muted); }
  td { padding: 12px; border-bottom: 1px solid #f1f5f9; font-size: 13px; }
  .badge { padding: 2px 8px; border-radius: 100px; font-size: 10px; font-weight: 600; }
  .badge-critical { background: #fef2f2; color: var(--critical); }
  .badge-low { background: #f0fdf4; color: var(--low); }
  .footer { text-align: center; margin-top: 40px; font-size: 11px; color: var(--muted); }
</style>
</head>
<body>
  <div class="container">
    <div class="report-header">
      <h1>OCULTAR Risk Assessment</h1>
      <div class="meta-grid">
        <div class="meta-item"><label>Report ID</label><span>OCU-{{.Meta.ReportID}}</span></div>
        <div class="meta-item"><label>Generated</label><span>{{.Meta.GeneratedAt}}</span></div>
        <div class="meta-item"><label>Engine</label><span>{{.Meta.EngineVersion}}</span></div>
      </div>
    </div>

    <div class="risk-banner {{.Risk.OverallRiskLevel}}">
      <div class="risk-dial">{{printf "%.1f" .Risk.OverallRiskScore}}</div>
      <div>
        <h2 style="font-size:18px; margin-bottom:4px;">{{.Risk.OverallRiskLevel}} Risk — {{if .Risk.IsGDPRPseudonymized}}Pseudonymized (Heuristic Assessment){{else}}Elevated Technical Risk Level{{end}}</h2>
        <p style="font-size:13px; opacity:0.7;">Estimated Var Range: <strong>€{{printf "%.0f" .Risk.Exposure.VaRMin}} - €{{printf "%.0f" .Risk.Exposure.VaRMax}}</strong></p>
      </div>
    </div>

    <div class="section">
      <h2>Risk Scorecard</h2>
      <table>
        <thead><tr><th>Category</th><th>Score</th><th>Level</th><th>Business Implication</th></tr></thead>
        <tbody>
          <tr>
            <td>Identifiability</td>
            <td>{{printf "%.1f" .Risk.Identifiability.Score}}</td>
            <td><span class="badge badge-{{lower .Risk.Identifiability.Label}}">{{.Risk.Identifiability.Label}}</span></td>
            <td>{{.Risk.Identifiability.Implication}}</td>
          </tr>
          <tr>
            <td>Financial Exposure</td>
            <td>{{printf "%.1f" .Risk.FinancialSensitivity.Score}}</td>
            <td><span class="badge badge-{{lower .Risk.FinancialSensitivity.Label}}">{{.Risk.FinancialSensitivity.Label}}</span></td>
            <td>{{.Risk.FinancialSensitivity.Implication}}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="section">
      <h2>Technical Metrics — Interpreted</h2>
      <div style="margin-bottom:20px;">
        <strong>K-Anonymity Score: {{.Risk.KAnonymity}}</strong><br>
        <p style="font-size:12px; color:var(--muted); margin-top:4px;">{{.Risk.KAnonymityInterpretation}}</p>
      </div>
      <div>
        <strong>L-Diversity Score: {{.Risk.LDiversity}}</strong><br>
        <p style="font-size:12px; color:var(--muted); margin-top:4px;">{{.Risk.LDiversityInterpretation}}</p>
      </div>
    </div>

    <div class="section">
      <h2>Financial Exposure — Three-Pillar VaR Model</h2>
      <p style="font-size:12px; color:var(--muted); margin-bottom:16px;">This model anchors technical risk scores to industry breach benchmarks (IBM/Ponemon) to simulate potential Value at Risk (VaR). All figures are projected ranges.</p>
      <table>
        <thead>
          <tr><th>Pillar / Component</th><th>Methodology</th><th style="text-align:right">Min Est. (€)</th><th style="text-align:right">Max Est. (€)</th></tr>
        </thead>
        <tbody>
          <tr>
            <td><strong>1. Regulatory Exposure</strong></td>
            <td>Simulation anchors (€10k-€100k) × Score</td>
            <td style="text-align:right">{{printf "%.0f" .Risk.Exposure.RegulatoryExposureMin}}</td>
            <td style="text-align:right">{{printf "%.0f" .Risk.Exposure.RegulatoryExposureMax}}</td>
          </tr>
          <tr>
            <td><strong>2. Operational Cost</strong></td>
            <td>Industry benchmarks (€100-€300/record)</td>
            <td style="text-align:right">{{printf "%.0f" .Risk.Exposure.OperationalCostMin}}</td>
            <td style="text-align:right">{{printf "%.0f" .Risk.Exposure.OperationalCostMax}}</td>
          </tr>
          <tr>
            <td><strong>3. Risk Multiplier</strong></td>
            <td>Profile-driven tiering (K/L profile)</td>
            <td style="text-align:right">{{printf "%.1f" .Risk.Exposure.RiskMultiplierMin}}×</td>
            <td style="text-align:right">{{printf "%.1f" .Risk.Exposure.RiskMultiplierMax}}×</td>
          </tr>
          <tr style="background:var(--bg); font-weight:700;">
            <td colspan="2">TOTAL VALUE AT RISK (SIMULATED RANGE)</td>
            <td style="text-align:right">€{{printf "%.0f" .Risk.Exposure.VaRMin}}</td>
            <td style="text-align:right; color:var(--critical);">€{{printf "%.0f" .Risk.Exposure.VaRMax}}</td>
          </tr>
        </tbody>
      </table>
      <p style="font-size:11px; color:var(--muted); margin-top:12px; line-height:1.4;">{{.Risk.Exposure.AssumptionsNote}}</p>
    </div>

    <div class="section">
      <h2>AI & LLM Exposure Assessment</h2>
      <table>
        <thead><tr><th>Parameter</th><th>Assessment / Guidance</th></tr></thead>
        <tbody>
          <tr><td><strong>Decision</strong></td><td style="font-weight:700; color:{{if eq .Risk.AI.Status "ALLOW"}}var(--low){{else}}var(--critical){{end}};">{{.Risk.AI.Status}}</td></tr>
          <tr><td><strong>External LLM API Safety</strong></td><td>{{.Risk.AI.LLMExposure}} Risk Profile</td></tr>
          <tr><td><strong>Vector DB / RAG Indexing</strong></td><td>{{if .Risk.AI.RAGSafe}}✅ Estimated safe for indexing{{else}}🚫 Sanitisation required before indexing{{end}}</td></tr>
        </tbody>
      </table>
      <div style="margin-top:16px; font-size:12px; border-left:4px solid var(--accent); padding-left:16px; color:var(--muted);">
        <strong>RAG Guidance:</strong> {{.Risk.AI.RAGGuidance}}
      </div>
    </div>

    <div class="section">
      <h2>Before / After Impact Simulation</h2>
      <table>
        <thead><tr><th>Metric</th><th>{{.Before.Label}}</th><th>{{.After.Label}}</th></tr></thead>
        <tbody>
          <tr><td><strong>Risk Level</strong></td><td><span class="badge badge-critical">{{.Before.RiskLevel}}</span></td><td><span class="badge badge-low">{{.After.RiskLevel}}</span></td></tr>
          <tr><td><strong>Risk Score</strong></td><td>{{.Before.RiskScore}}</td><td>{{.After.RiskScore}}</td></tr>
          <tr><td><strong>VaR Range</strong></td><td>{{.Before.VaRRange}}</td><td>{{.After.VaRRange}}</td></tr>
        </tbody>
      </table>
    </div>

    <div class="section">
      <h2>Structured Remediation Plan</h2>
      <div style="font-size:13px; color:var(--text); white-space:pre-wrap; line-height:1.6;">{{.Risk.Recommendation}}</div>
    </div>

    <div class="footer">
      Generated automatically by OCULTAR. Methodology v{{.Meta.MethodologyVersion}}<br>
      © 2026 Hector Eduardo Trejos Cabezas. Licensed under Apache 2.0.
    </div>
  </div>
</body>
</html>`
