import { escapeHtml } from "../utils/html.js";

export function showMsg(message, isError = false, durationMs = 3000) {
  const container = document.getElementById("toast-container");

  container.innerHTML = "";

  const toast = document.createElement("div");
  toast.className = `toast ${isError ? "error" : "success"}`;
  toast.innerHTML = `
        <span>${escapeHtml(message)}</span>
        <button class="toast-close" title="Tutup">&times;</button>
    `;

  const dismiss = () => {
    toast.style.animation = "toast-out 0.2s ease-in forwards";
    setTimeout(() => toast.remove(), 200);
  };
  toast.querySelector(".toast-close").addEventListener("click", dismiss);

  container.appendChild(toast);

  setTimeout(
    () => {
      if (toast.parentNode) dismiss();
    },
    Math.max(1000, durationMs),
  );
}
