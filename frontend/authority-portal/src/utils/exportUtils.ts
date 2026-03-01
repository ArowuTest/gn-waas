/**
 * GN-WAAS Export Utilities
 *
 * Provides CSV and PDF export for NRW reports, account lists, and audit events.
 * All exports run client-side — no server round-trip needed.
 */

// ─── CSV Export ───────────────────────────────────────────────────────────────

export interface CSVColumn<T> {
  header: string
  accessor: (row: T) => string | number | null | undefined
}

/**
 * Export an array of objects to a CSV file and trigger browser download
 */
export function exportToCSV<T>(
  data: T[],
  columns: CSVColumn<T>[],
  filename: string
): void {
  if (data.length === 0) {
    alert('No data to export')
    return
  }

  const headers = columns.map(c => `"${c.header}"`).join(',')
  const rows = data.map(row =>
    columns
      .map(c => {
        const val = c.accessor(row)
        if (val == null) return '""'
        const str = String(val).replace(/"/g, '""')
        return `"${str}"`
      })
      .join(',')
  )

  const csv = [headers, ...rows].join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8;' })
  downloadBlob(blob, filename.endsWith('.csv') ? filename : `${filename}.csv`)
}

// ─── PDF Export (print-based) ─────────────────────────────────────────────────

export interface PDFExportOptions {
  title: string
  subtitle?: string
  generatedBy?: string
  logoText?: string
}

/**
 * Export a table to PDF using the browser's print dialog.
 * Opens a new window with a styled print-ready HTML page.
 */
export function exportTableToPDF<T>(
  data: T[],
  columns: CSVColumn<T>[],
  options: PDFExportOptions
): void {
  if (data.length === 0) {
    alert('No data to export')
    return
  }

  const now = new Date().toLocaleString('en-GH', {
    dateStyle: 'long',
    timeStyle: 'short',
  })

  const tableRows = data
    .map(
      row =>
        `<tr>${columns
          .map(c => {
            const val = c.accessor(row)
            return `<td>${val ?? '—'}</td>`
          })
          .join('')}</tr>`
    )
    .join('')

  const html = `
    <!DOCTYPE html>
    <html>
    <head>
      <meta charset="utf-8" />
      <title>${options.title}</title>
      <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: 'Segoe UI', Arial, sans-serif; font-size: 11px; color: #111; padding: 20px; }
        .header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; border-bottom: 2px solid #2e7d32; padding-bottom: 12px; }
        .logo { font-size: 18px; font-weight: 800; color: #2e7d32; }
        .logo span { color: #1565c0; }
        .title-block h1 { font-size: 16px; font-weight: 700; color: #111; }
        .title-block p { font-size: 11px; color: #6b7280; margin-top: 4px; }
        .meta { font-size: 10px; color: #6b7280; text-align: right; }
        table { width: 100%; border-collapse: collapse; margin-top: 16px; }
        th { background: #2e7d32; color: #fff; padding: 8px 10px; text-align: left; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; }
        td { padding: 7px 10px; border-bottom: 1px solid #e5e7eb; font-size: 10px; }
        tr:nth-child(even) td { background: #f9fafb; }
        .footer { margin-top: 20px; font-size: 9px; color: #9ca3af; text-align: center; border-top: 1px solid #e5e7eb; padding-top: 10px; }
        @media print {
          body { padding: 10px; }
          @page { margin: 15mm; size: A4 landscape; }
        }
      </style>
    </head>
    <body>
      <div class="header">
        <div>
          <div class="logo">GN-<span>WAAS</span></div>
          <div class="title-block">
            <h1>${options.title}</h1>
            ${options.subtitle ? `<p>${options.subtitle}</p>` : ''}
          </div>
        </div>
        <div class="meta">
          Generated: ${now}<br/>
          ${options.generatedBy ? `By: ${options.generatedBy}` : ''}
        </div>
      </div>
      <table>
        <thead>
          <tr>${columns.map(c => `<th>${c.header}</th>`).join('')}</tr>
        </thead>
        <tbody>${tableRows}</tbody>
      </table>
      <div class="footer">
        Ghana National Water Audit &amp; Assurance System (GN-WAAS) — Confidential — ${now}
      </div>
      <script>window.onload = () => { window.print(); }</script>
    </body>
    </html>
  `

  const win = window.open('', '_blank')
  if (!win) {
    alert('Please allow popups to export PDF')
    return
  }
  win.document.write(html)
  win.document.close()
}

// ─── Typed Export Helpers ─────────────────────────────────────────────────────

export function exportNRWSummaryCSV(districts: any[]): void {
  exportToCSV(
    districts,
    [
      { header: 'District', accessor: r => r.district_name },
      { header: 'System Input (m³)', accessor: r => r.system_input_m3?.toFixed(1) },
      { header: 'Billed (m³)', accessor: r => r.billed_m3?.toFixed(1) },
      { header: 'NRW (m³)', accessor: r => r.nrw_m3?.toFixed(1) },
      { header: 'NRW %', accessor: r => r.nrw_percent?.toFixed(1) + '%' },
      { header: 'IWA Grade', accessor: r => r.iwa_grade },
      { header: 'ILI', accessor: r => r.ili?.toFixed(2) },
      { header: 'Est. Recovery (GHS)', accessor: r => r.estimated_recovery_ghs?.toFixed(2) },
      { header: 'Open Flags', accessor: r => r.open_flags },
      { header: 'Period', accessor: r => r.period },
    ],
    `GN-WAAS_NRW_Summary_${new Date().toISOString().slice(0, 10)}`
  )
}

export function exportNRWSummaryPDF(districts: any[], subtitle?: string): void {
  exportTableToPDF(
    districts,
    [
      { header: 'District', accessor: r => r.district_name },
      { header: 'System Input (m³)', accessor: r => r.system_input_m3?.toFixed(1) },
      { header: 'NRW (m³)', accessor: r => r.nrw_m3?.toFixed(1) },
      { header: 'NRW %', accessor: r => r.nrw_percent?.toFixed(1) + '%' },
      { header: 'IWA Grade', accessor: r => r.iwa_grade },
      { header: 'ILI', accessor: r => r.ili?.toFixed(2) },
      { header: 'Est. Recovery (GHS)', accessor: r => r.estimated_recovery_ghs?.toFixed(2) },
      { header: 'Open Flags', accessor: r => r.open_flags },
    ],
    {
      title: 'NRW Summary Report',
      subtitle: subtitle ?? `Generated ${new Date().toLocaleDateString('en-GH')}`,
      logoText: 'GN-WAAS',
    }
  )
}

export function exportAccountsCSV(accounts: any[]): void {
  exportToCSV(
    accounts,
    [
      { header: 'Account Number', accessor: r => r.gwl_account_number },
      { header: 'Customer Name', accessor: r => r.account_holder_name },
      { header: 'Category', accessor: r => r.account_category },
      { header: 'District', accessor: r => r.district_name },
      { header: 'Address', accessor: r => r.address_line1 },
      { header: 'Meter Serial', accessor: r => r.meter_serial_number },
      { header: 'Status', accessor: r => r.status },
      { header: 'Avg Consumption (m³)', accessor: r => r.avg_monthly_consumption_m3?.toFixed(1) },
    ],
    `GN-WAAS_Accounts_${new Date().toISOString().slice(0, 10)}`
  )
}

export function exportAuditEventsCSV(events: any[]): void {
  exportToCSV(
    events,
    [
      { header: 'Reference', accessor: r => r.audit_reference },
      { header: 'Account', accessor: r => r.account_number },
      { header: 'Customer', accessor: r => r.customer_name },
      { header: 'District', accessor: r => r.district_name },
      { header: 'Anomaly Type', accessor: r => r.anomaly_type },
      { header: 'Alert Level', accessor: r => r.alert_level },
      { header: 'Status', accessor: r => r.status },
      { header: 'GWL Bill (GHS)', accessor: r => r.gwl_billed_ghs?.toFixed(2) },
      { header: 'Shadow Bill (GHS)', accessor: r => r.shadow_bill_ghs?.toFixed(2) },
      { header: 'Variance %', accessor: r => r.variance_pct?.toFixed(1) + '%' },
      { header: 'Recovery (GHS)', accessor: r => r.recovery_amount_ghs?.toFixed(2) },
      { header: 'GRA QR Code', accessor: r => r.gra_qr_code_url ?? 'Pending' },
      { header: 'Created At', accessor: r => r.created_at ? new Date(r.created_at).toLocaleDateString('en-GH') : '' },
    ],
    `GN-WAAS_Audit_Events_${new Date().toISOString().slice(0, 10)}`
  )
}

// ─── Utility ──────────────────────────────────────────────────────────────────

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
