export async function fetchRemindersApi(params, signal, etag) {
  let url = `/api/reminders?limit=${params.limit || 20}`;
  if (params.cursor) url += `&cursor=${params.cursor}`;
  if (params.search) url += `&search=${encodeURIComponent(params.search)}`;
  if (params.sortBy)
    url += `&sortBy=${params.sortBy}&order=${params.sortOrder}`;

  const headers = {};
  if (etag) headers["If-None-Match"] = etag;

  const res = await fetch(url, { signal, headers });
  return res;
}

export async function createReminderApi(payload) {
  const res = await fetch("/api/reminders", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error("Failed to create reminder");
  return res;
}

export async function deleteReminderApi(id) {
  const res = await fetch(`/api/reminders/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("Failed to delete reminder");
  return res;
}

export async function deleteAllRemindersApi() {
  const res = await fetch("/api/reminders", { method: "DELETE" });
  if (!res.ok) throw new Error("Failed to delete all reminders");
  return res;
}

export async function toggleReminderApi(id) {
  const res = await fetch(`/api/reminders/${id}/toggle`, { method: "PATCH" });
  if (!res.ok) throw new Error("Failed to toggle reminder");
  return res;
}

export async function updateReminderApi(id, payload) {
  const res = await fetch(`/api/reminders/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.text();
    throw new Error(err);
  }
  return res;
}
