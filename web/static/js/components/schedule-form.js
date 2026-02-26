import { t, fpLocaleId, currentLang } from "../i18n/lang.js";
import { showMsg } from "./toast.js";
import { state, globals } from "../store/state.js";
import { loadReminders } from "./reminders-table.js";
import { htmlToWAMarkdown } from "../utils/html.js";
import { cronToText, cronNextTime } from "../utils/format.js";
import { renderTargetChips } from "./target-chips.js";
import { pruneMessageEditors } from "./message-editor.js";

const scheduleForm = document.getElementById("schedule-form");
const timeContainer = document.getElementById("time-container");
const cronInput = document.getElementById("recurrence");
const cronPreview = document.getElementById("cron-preview");

export let timePicker;
let schedulePickerOutsideBound = false;

function isPickerInternalTarget(target, picker) {
  if (!picker || !target) return false;
  const calendar = picker.calendarContainer;
  const input = picker.input;
  const altInput = picker.altInput;

  return (
    (calendar && calendar.contains(target)) ||
    (input && (input === target || input.contains(target))) ||
    (altInput && (altInput === target || altInput.contains(target)))
  );
}

export function initScheduleForm() {
  timePicker = flatpickr("#time", {
    enableTime: true,
    time_24hr: true,
    dateFormat: "Y-m-d H:i",
    altInput: true,
    altFormat: "j F Y, H:i",
    minDate: "today",
    disableMobile: true,
    appendTo: document.body,
    locale: currentLang === "id" ? fpLocaleId : "default",
  });

  if (!schedulePickerOutsideBound) {
    document.addEventListener(
      "pointerdown",
      (e) => {
        if (!timePicker || !timePicker.isOpen) return;
        if (isPickerInternalTarget(e.target, timePicker)) return;
        timePicker.close();
      },
      true,
    );
    schedulePickerOutsideBound = true;
  }

  function updateTimeVisibility() {
    if (!timeContainer || !cronInput) return;
    const hasCron = cronInput.value.trim() !== "";
    timeContainer.hidden = hasCron;
  }

  document.querySelectorAll(".cron-preset").forEach((btn) => {
    btn.addEventListener("click", () => {
      cronInput.value = btn.dataset.cron;
      const expr = btn.dataset.cron;
      const text = cronToText(expr, true);
      const next = cronNextTime(expr);
      cronPreview.textContent = next ? `${text} — ${next}` : text;
      updateTimeVisibility();
    });
  });

  if (cronInput)
    cronInput.addEventListener("input", () => {
      const expr = cronInput.value.trim();
      const text = cronToText(expr, true);
      const next = cronNextTime(expr);
      cronPreview.textContent = next ? `${text} — ${next}` : text;
      updateTimeVisibility();
    });

  if (scheduleForm)
    scheduleForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (!state.wa_number) {
        showMsg(t("waNotLinked"), true);
        return;
      }

      const targetWa = globals.targetNumbers.join(",");
      const btn = document.getElementById("schedule-btn");

      try {
        btn.disabled = true;
        const messages = [];

        pruneMessageEditors();

        Object.keys(globals.messageEditors).forEach((id) => {
          const quill = globals.messageEditors[id];

          if (document.getElementById(`message-container-${id}`)) {
            const val = htmlToWAMarkdown(quill.root.innerHTML);
            if (val !== "") {
              messages.push(val);
            }
          }
        });

        if (messages.length === 0) {
          showMsg(t("enterMessage"), true);
          btn.disabled = false;
          return;
        }

        const recurrence = cronInput.value;
        const timeVal = document.getElementById("time").value;
        const hasCron = recurrence.trim() !== "";

        if (!hasCron && !timeVal) {
          showMsg(t("selectTime"), true);
          btn.disabled = false;
          return;
        }

        const isoDate = hasCron
          ? new Date().toISOString()
          : new Date(timeVal).toISOString();

        let successCount = 0;
        for (const msg of messages) {
          const res = await fetch("/api/reminders", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              message: msg,
              target_wa: targetWa,
              recurrence,
              scheduled_at: isoDate,
            }),
          });
          if (res.ok) successCount++;
        }

        if (successCount === messages.length) {
          showMsg(`${successCount} ${t("scheduled")}`);
        } else {
          showMsg(
            `${t("partialFail")} ${successCount}/${messages.length}`,
            true,
          );
        }

        scheduleForm.reset();

        const messageList = document.getElementById("message-list");
        const blocks = messageList.querySelectorAll(".message-block");
        for (let i = 1; i < blocks.length; i++) {
          blocks[i].remove();
        }
        const removeBtn = messageList.querySelector(".remove-message-btn");
        if (removeBtn) removeBtn.style.display = "none";
        const lbl = messageList.querySelector("label");
        if (lbl)
          lbl.innerHTML = `<span data-i18n="messageLabel">${t("messageLabel")}</span> 1:`;
        globals.messageCount = 1;
        pruneMessageEditors();

        if (cronPreview)
          cronPreview.textContent = "Kosongkan untuk hanya sekali.";
        timePicker.clear();
        globals.targetNumbers = [];
        renderTargetChips();
        state.lastETag = null;
        loadReminders(true);
      } catch (err) {
        showMsg(err.message, true);
      } finally {
        btn.disabled = false;
      }
    });
}
