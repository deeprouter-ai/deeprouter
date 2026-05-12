# Phase 1 Task — Add 4 Airbotix fields to Admin UI

> **Plan reference**: [`PLAN.md`](../../PLAN.md) Phase 1 (Week 3-4) · "Admin UI fields"
> **Status**: ⏳ Not started
> **Estimate**: 1-2 days for a frontend engineer familiar with React + react-hook-form + Zod

---

## Goal

Operators creating/editing a User in the admin UI can set the 4 Airbotix fields (`kids_mode`, `policy_profile`, `billing_webhook_url`, `custom_pricing_id`) and see them persist. No new API endpoint needed — the existing user PUT route accepts the JSON fields once the backend DTO is updated.

---

## File-by-file changes

### Backend — Go

#### `controller/user.go`
Find the user update handler (likely `UpdateUser` or `EditUser`).
Add the 4 fields to the request body schema struct (if separate from `model.User`).
If it already uses `model.User` directly, no backend changes needed for the basic fields — GORM picks them up.

**Verify**:
```bash
docker compose exec postgres psql -U root -d new-api -c "\d users" | grep -E "kids_mode|policy_profile|billing_webhook|custom_pricing"
```
Should show all 4 columns after `docker compose restart new-api`.

---

### Frontend — React/TS (web/default)

Stack: **Rsbuild + React + TanStack Router + react-hook-form + Zod + shadcn/ui**

#### A. `web/default/src/features/users/types.ts`

Extend `userSchema`:
```ts
export const userSchema = z.object({
  id: z.number(),
  username: z.string(),
  // ... existing fields ...

  // --- Airbotix additions ---
  kids_mode: z.boolean().default(false),
  policy_profile: z.enum(['passthrough', 'adult', 'kid-safe']).default('passthrough'),
  billing_webhook_url: z.string().url().optional().or(z.literal('')),
  custom_pricing_id: z.string().optional(),
})
```

#### B. `web/default/src/features/users/lib/user-form.ts`

Add to `userFormSchema`:
```ts
export const userFormSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  // ... existing fields ...

  // --- Airbotix additions ---
  kids_mode: z.boolean().optional().default(false),
  policy_profile: z.enum(['passthrough', 'adult', 'kid-safe']).optional().default('passthrough'),
  billing_webhook_url: z.string().url().optional().or(z.literal('')),
  custom_pricing_id: z.string().optional(),
})
```

Add to `USER_FORM_DEFAULT_VALUES`:
```ts
export const USER_FORM_DEFAULT_VALUES: UserFormValues = {
  username: '',
  display_name: '',
  // ... existing ...

  // --- Airbotix additions ---
  kids_mode: false,
  policy_profile: 'passthrough',
  billing_webhook_url: '',
  custom_pricing_id: '',
}
```

Update the `formDataToApiPayload` function (or equivalent) to include these fields in the payload.

#### C. `web/default/src/features/users/components/users-mutate-drawer.tsx`

Add four `<FormField>` entries inside the existing form, grouped in a section "Airbotix tenant settings":

```tsx
{/* --- Airbotix tenant settings --- */}
<div className="border-t pt-4 mt-4">
  <h3 className="text-sm font-medium mb-3">Airbotix tenant settings</h3>

  <FormField
    control={form.control}
    name="kids_mode"
    render={({ field }) => (
      <FormItem className="flex flex-row items-center justify-between rounded-lg border p-3 mb-3">
        <div className="space-y-0.5">
          <FormLabel>Kids mode</FormLabel>
          <FormDescription>
            Forces strict kid-safe policy (overrides profile, strips PII, enforces ZDR, model whitelist).
          </FormDescription>
        </div>
        <FormControl>
          <Switch checked={field.value} onCheckedChange={field.onChange} />
        </FormControl>
      </FormItem>
    )}
  />

  <FormField
    control={form.control}
    name="policy_profile"
    render={({ field }) => (
      <FormItem className="mb-3">
        <FormLabel>Policy profile</FormLabel>
        <Select onValueChange={field.onChange} defaultValue={field.value}>
          <FormControl>
            <SelectTrigger><SelectValue placeholder="Pick a profile" /></SelectTrigger>
          </FormControl>
          <SelectContent>
            <SelectItem value="passthrough">passthrough</SelectItem>
            <SelectItem value="adult">adult</SelectItem>
            <SelectItem value="kid-safe">kid-safe</SelectItem>
          </SelectContent>
        </Select>
        <FormDescription>
          Behavioural profile. Overridden by Kids mode when that is on.
        </FormDescription>
        <FormMessage />
      </FormItem>
    )}
  />

  <FormField
    control={form.control}
    name="billing_webhook_url"
    render={({ field }) => (
      <FormItem className="mb-3">
        <FormLabel>Billing webhook URL</FormLabel>
        <FormControl>
          <Input placeholder="https://api.kidsinai.org/internal/deeprouter/billing" {...field} />
        </FormControl>
        <FormDescription>
          DeepRouter POSTs per-request billing events here. Empty = no webhook.
        </FormDescription>
        <FormMessage />
      </FormItem>
    )}
  />

  <FormField
    control={form.control}
    name="custom_pricing_id"
    render={({ field }) => (
      <FormItem>
        <FormLabel>Custom pricing ID</FormLabel>
        <FormControl>
          <Input placeholder="(optional)" {...field} />
        </FormControl>
        <FormMessage />
      </FormItem>
    )}
  />
</div>
```

You may need to import: `Switch`, `Select`, `SelectTrigger`, `SelectContent`, `SelectItem`, `SelectValue` from `@/components/ui/...`.

#### D. `web/default/src/features/users/components/users-columns.tsx`

Add a column for `kids_mode` (boolean badge) so it's visible in the table list:
```tsx
{
  accessorKey: 'kids_mode',
  header: 'Kids',
  cell: ({ row }) =>
    row.original.kids_mode
      ? <Badge variant="default">kids_mode</Badge>
      : null,
}
```

---

## i18n strings (optional but recommended)

Add labels to `web/default/src/locales/en/users.json` (and the zh-CN, zh-TW, fr, ja siblings):
```json
{
  "users": {
    "form": {
      "airbotix": {
        "section": "Airbotix tenant settings",
        "kids_mode_label": "Kids mode",
        "kids_mode_description": "Forces strict kid-safe policy...",
        "policy_profile_label": "Policy profile",
        "billing_webhook_url_label": "Billing webhook URL",
        "custom_pricing_id_label": "Custom pricing ID"
      }
    }
  }
}
```

---

## Acceptance checklist

- [ ] `pnpm typecheck` passes in `web/default/`
- [ ] `bun run dev` (or `pnpm dev`) loads admin UI; user edit drawer shows the new section
- [ ] Toggling `kids_mode` and saving → reload admin UI → toggle persists
- [ ] Same for policy_profile / billing_webhook_url / custom_pricing_id
- [ ] `psql ... SELECT kids_mode, policy_profile, billing_webhook_url FROM users WHERE id=X` returns the saved values
- [ ] [`docs/tenant-onboarding.md`](../tenant-onboarding.md) step 3 SQL workaround can be deleted (UI handles it)

---

## Rebase strategy

Keep the diff localised — these are the touched files:
- `web/default/src/features/users/types.ts` (3-4 lines added)
- `web/default/src/features/users/lib/user-form.ts` (10 lines added)
- `web/default/src/features/users/components/users-mutate-drawer.tsx` (~70 lines added in 1 section)
- `web/default/src/features/users/components/users-columns.tsx` (8 lines added)
- `web/default/src/locales/*/users.json` (1 key block each)

Total: ~100 lines added across 5+ files; **no deletes from upstream code**. Cherry-pick from upstream stays clean.

---

## Out of scope (deferred to later phases)

- [ ] Wiring `kids_mode` into the relay path → Phase 2 of [`PLAN.md`](../../PLAN.md)
- [ ] Validation: `kids_mode=true` should also force `policy_profile=kid-safe` → enforce at submit
- [ ] Audit log of who toggled `kids_mode` → V1
