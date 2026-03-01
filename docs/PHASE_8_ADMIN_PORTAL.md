# Phase 8 — Admin Portal Documentation

## Tech Stack
- **React 19** + **TypeScript** (strict mode)
- **Vite 5** (build tool)
- **Tailwind CSS 3** (utility-first styling)
- **React Router v6** (client-side routing)
- **TanStack Query v5** (server state management)
- **Recharts** (data visualisation)
- **Lucide React** (icons)
- **Axios** (HTTP client)
- **React Hook Form + Zod** (form validation)

## Design System

### Colour Palette
```
Brand Green:  #2e7d32 (Ghana national colour)
Ghana Gold:   #fdd835 (accent)
Danger:       #dc2626
Warning:      #d97706
Success:      #16a34a
Info:         #2563eb
```

### Component Classes (Tailwind)
```css
.card          → White rounded card with shadow
.btn-primary   → Brand green button
.btn-secondary → White outlined button
.btn-danger    → Red destructive button
.badge-red     → Critical/danger badge
.badge-yellow  → Warning/pending badge
.badge-green   → Success/resolved badge
.badge-blue    → Info/assigned badge
.input         → Form input with focus ring
.table         → Styled data table
.sidebar-link  → Navigation link with active state
```

## Pages

### Dashboard (`/dashboard`)
- 8 KPI stat cards (anomalies, audits, revenue loss, success fees)
- Anomaly severity pie chart (Recharts)
- Recent open anomalies table
- IWA/AWWA Water Balance framework banner
- District selector filter

### Anomaly Flags (`/anomalies`)
- Full anomaly list with multi-filter (district, level, type, status)
- Trigger Sentinel Scan button
- Pagination
- Click-through to anomaly detail

### Audit Events (`/audits`)
- Audit list with status + variance display
- Colour-coded variance (red if >15%)
- GRA status badges
- New Audit creation

### NRW Analysis (`/nrw`)
- District NRW bar chart with IWA target line (20%) and Ghana average line (51.6%)
- Colour-coded bars (green <20%, amber 20-40%, red >40%)
- District summary table with Data Confidence Grade
- IWA/AWWA Water Balance component reference cards

### Login (`/login`)
- Email/password form
- Dev mode quick-access buttons (4 roles)
- Keycloak integration ready

## Authentication Flow
1. User submits credentials → `POST /api/v1/auth/login`
2. API Gateway validates with Keycloak
3. JWT token stored in `localStorage`
4. All requests include `Authorization: Bearer <token>`
5. 401 response → redirect to `/login`

## RBAC in Frontend
```typescript
const { hasRole } = useAuth()

// Conditional rendering
{hasRole('SYSTEM_ADMIN', 'AUDIT_SUPERVISOR') && (
  <button>Assign Officer</button>
)}

// Route-level protection
<Route path="/settings" element={
  <RequireRole roles={['SYSTEM_ADMIN']}>
    <SettingsPage />
  </RequireRole>
} />
```

## Running Locally
```bash
cd apps/admin-portal
npm install
npm run dev
# Opens at http://localhost:5173
```

## Building for Production
```bash
npm run build
# Output in dist/
```

## Docker
```bash
docker build -t gnwaas-admin-portal .
docker run -p 80:80 gnwaas-admin-portal
```
