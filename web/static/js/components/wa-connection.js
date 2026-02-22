import { t } from "../i18n/lang.js";
import { showMsg } from "./toast.js";
import { state, globals } from "../store/state.js";
import { showLogin } from "./auth.js";
import { createQrEventSource } from "../api/whatsapp.js";

const waStatus = document.getElementById("wa-status");
const waLinkActions = document.getElementById("wa-link-actions");
const linkWaQrBtn = document.getElementById("link-wa-qr-btn");
const linkWaPhoneBtn = document.getElementById("link-wa-phone-btn");
const waPhoneInput = document.getElementById("wa-phone");
const qrContainer = document.getElementById("qr-container");
const qrImg = document.getElementById("qr-img");
const codeContainer = document.getElementById("code-container");
const pairCodeDisplay = document.getElementById("pair-code-display");
const unlinkWaBtn = document.getElementById("unlink-wa-btn");
const pickGroupBtn = document.getElementById("pick-group-btn");
const pickContactBtn = document.getElementById("pick-contact-btn");

export function updateDash() {
  const editPickGroupBtn = document.getElementById("edit-pick-group-btn");
  const editPickContactBtn = document.getElementById("edit-pick-contact-btn");
  if (state.wa_number) {
    waStatus.textContent = `${t("waConnected")} ${state.wa_number}`;
    waLinkActions.hidden = true;
    qrContainer.hidden = true;
    codeContainer.hidden = true;
    if (pickGroupBtn) pickGroupBtn.disabled = false;
    if (pickContactBtn) pickContactBtn.disabled = false;
    if (editPickGroupBtn) editPickGroupBtn.disabled = false;
    if (editPickContactBtn) editPickContactBtn.disabled = false;
    if (unlinkWaBtn) unlinkWaBtn.hidden = false;
  } else {
    waStatus.textContent = t("waNotConnected");
    waLinkActions.hidden = false;
    if (pickGroupBtn) pickGroupBtn.disabled = true;
    if (pickContactBtn) pickContactBtn.disabled = true;
    if (editPickGroupBtn) editPickGroupBtn.disabled = true;
    if (editPickContactBtn) editPickContactBtn.disabled = true;
    if (unlinkWaBtn) unlinkWaBtn.hidden = true;
  }
}

export async function initWA() {
  try {
    const res = await fetch("/api/wa/status");
    if (res.status === 401) {
      showLogin();
      return;
    }
    if (res.ok) {
      const data = await res.json();
      if (data.status === "connected") {
        state.wa_number = data.number;
        localStorage.setItem("rm_wa_number", data.number);
      } else {
        state.wa_number = null;
        localStorage.removeItem("rm_wa_number");
      }
    }
  } catch (e) {
    console.error("Failed to get WA status", e);
  }
  updateDash();
}

export function initWaConnection() {
  if (linkWaQrBtn)
    linkWaQrBtn.addEventListener("click", () => {
      qrContainer.hidden = false;
      codeContainer.hidden = true;
      qrImg.src = "";
      qrImg.alt = "Menghasilkan QR...";
      linkWaQrBtn.disabled = true;

      const evtSource = createQrEventSource();

      evtSource.onmessage = (e) => {
        const data = JSON.parse(e.data);
        if (data.type === "qr") {
          if (data.image) {
            qrImg.src = data.image;
            qrImg.alt = "QR Code";
          } else {
            qrImg.src = "";
            qrImg.alt = "QR unavailable";
          }
        } else if (data.type === "success") {
          state.wa_number = data.number;
          localStorage.setItem("rm_wa_number", data.number);
          showMsg(`${t("waLinked")} ${data.number}`);
          evtSource.close();
          updateDash();
        } else if (data.type === "error") {
          showMsg(data.message, true);
          evtSource.close();
          linkWaQrBtn.disabled = false;
          qrContainer.hidden = true;
        }
      };

      evtSource.onerror = () => {
        showMsg(t("waConnectionLost"), true);
        evtSource.close();
        linkWaQrBtn.disabled = false;
      };
    });

  if (linkWaPhoneBtn)
    linkWaPhoneBtn.addEventListener("click", () => {
      const phone = waPhoneInput.value.trim();
      if (!phone) {
        showMsg(t("waEnterPhone"), true);
        return;
      }

      codeContainer.hidden = false;
      qrContainer.hidden = true;
      pairCodeDisplay.textContent = "Menghasilkan...";
      linkWaPhoneBtn.disabled = true;

      const evtSource = new EventSource(
        `/api/wa/pair?phone=${encodeURIComponent(phone)}`,
      );

      evtSource.onmessage = (e) => {
        const data = JSON.parse(e.data);
        if (data.type === "code") {
          pairCodeDisplay.textContent = data.code;
        } else if (data.type === "success") {
          state.wa_number = data.number;
          localStorage.setItem("rm_wa_number", data.number);
          showMsg(`${t("waLinked")} ${data.number}`);
          evtSource.close();
          updateDash();
        } else if (data.type === "error") {
          showMsg(data.message, true);
          evtSource.close();
          linkWaPhoneBtn.disabled = false;
          codeContainer.hidden = true;
        }
      };

      evtSource.onerror = () => {
        showMsg(t("waConnectionLost"), true);
        evtSource.close();
        linkWaPhoneBtn.disabled = false;
      };
    });

  if (unlinkWaBtn)
    unlinkWaBtn.addEventListener("click", async () => {
      if (!window.confirm("Putuskan koneksi WhatsApp?")) return;
      try {
        const res = await fetch("/api/wa", { method: "DELETE" });
        if (res.ok) {
          state.wa_number = null;
          localStorage.removeItem("rm_wa_number");
          updateDash();
          showMsg(t("waUnlinked"));
        }
      } catch (err) {
        showMsg(t("waUnlinkFailed"), true);
      }
    });
}
