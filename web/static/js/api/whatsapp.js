export async function fetchWaGroups() {
  const res = await fetch("/api/wa/groups");
  if (!res.ok) throw new Error("Failed to load groups");
  return res.json();
}

export async function fetchWaContacts() {
  const res = await fetch("/api/wa/contacts");
  if (!res.ok) throw new Error("Failed to load contacts");
  return res.json();
}

export async function unlinkWa() {
  const res = await fetch("/api/wa", { method: "DELETE" });
  if (!res.ok) throw new Error("Failed to unlink");
  return true;
}

export async function getWaStatus() {
  const res = await fetch("/api/wa/status");
  if (res.status === 401) return { status: "unauthorized" };
  if (!res.ok) throw new Error("Failed to get status");
  return res.json();
}

export function createQrEventSource() {
  return new EventSource(`/api/wa/qr`);
}

export function createPairEventSource(phone) {
  return new EventSource(`/api/wa/pair?phone=${encodeURIComponent(phone)}`);
}
