export function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

export function formatWhatsAppMarkdown(text) {
  if (!text) return "";
  let html = escapeHtml(text);
  html = html.replace(/\*([^]+?)\*/g, "<b>$1</b>");
  html = html.replace(/_([^]+?)_/g, "<i>$1</i>");
  html = html.replace(/~([^]+?)~/g, "<s>$1</s>");
  html = html.replace(
    /```([^]+?)```/g,
    '<span style="font-family: monospace; background: var(--btn-bg); padding: 2px 4px; border-radius: 3px;">$1</span>',
  );
  return html;
}

export function htmlToWAMarkdown(html) {
  let text = html;
  text = text.replace(/<(strong|b)>(.*?)<\/\1>/gi, "*$2*");
  text = text.replace(/<(em|i)>(.*?)<\/\1>/gi, "_$2_");
  text = text.replace(/<(s|strike)>(.*?)<\/\1>/gi, "~$2~");

  text = text.replace(/<pre[^>]*>([\s\S]*?)<\/pre>/gi, function (match, inner) {
    return "```\n" + inner + "\n```\n";
  });
  text = text.replace(/<code[^>]*>([\s\S]*?)<\/code>/gi, "```$1```");

  text = text.replace(/<p><br><\/p>/g, "\n");
  text = text.replace(/<\/p><p>/g, "\n");
  text = text.replace(/<br>/g, "\n");

  text = text.replace(/<[^>]+>/g, "");

  let txt = document.createElement("textarea");
  txt.innerHTML = text;
  let decoded = txt.value;

  return decoded.trim();
}
