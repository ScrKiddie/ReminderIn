export function isValidWaFormat(val) {
  if (!val) return false;
  if (/^\d{6,15}$/.test(val)) return true;
  if (/^\d+-\d+(@g\.us)?$/.test(val)) return true;
  if (/@(s\.whatsapp\.net|g\.us|broadcast)$/.test(val)) return true;
  return false;
}
