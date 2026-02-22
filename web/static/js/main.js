import { t } from "./i18n/lang.js";
import { escapeHtml } from "./utils/html.js";

import { initThemeToggle } from "./components/theme.js";
import { initAuth, bootApp } from "./components/auth.js";
import { initWaConnection } from "./components/wa-connection.js";
import { initPickerModal } from "./components/picker-modal.js";
import { initTargetChips } from "./components/target-chips.js";
import { initMessageEditor, initQuill } from "./components/message-editor.js";
import { initScheduleForm } from "./components/schedule-form.js";
import { initTableInteractions } from "./components/reminders-table.js";
import { initDeleteModal } from "./components/delete-modal.js";
import { initEditModal } from "./components/edit-modal.js";
import { globals } from "./store/state.js";

window.t = t;
window.escapeHtml = escapeHtml;

window.setSort = typeof setSort !== "undefined" ? setSort : null;
window.toggleReminder =
  typeof toggleReminder !== "undefined" ? toggleReminder : null;
window.deleteReminder =
  typeof deleteReminder !== "undefined" ? deleteReminder : null;
window.editReminder = typeof editReminder !== "undefined" ? editReminder : null;
window.removeTarget = typeof removeTarget !== "undefined" ? removeTarget : null;
window.removeEditTarget =
  typeof removeEditTarget !== "undefined" ? removeEditTarget : null;

document.addEventListener("DOMContentLoaded", async () => {
  const editorMod = await import("./components/message-editor.js");
  globals.editQuill = editorMod.createLiteEditor("edit-message-container");
  editorMod.initQuill(0);

  initThemeToggle();
  initTargetChips();
  initPickerModal();
  initMessageEditor();
  initScheduleForm();
  initTableInteractions();
  initDeleteModal();
  initEditModal();
  initWaConnection();

  initAuth();
  bootApp();
});
