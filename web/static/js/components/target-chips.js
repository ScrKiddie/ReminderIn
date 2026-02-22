import { t } from "../i18n/lang.js";
import { escapeHtml } from "../utils/html.js";
import { showMsg } from "./toast.js";
import { globals } from "../store/state.js";
import { isValidWaFormat } from "../utils/validators.js";

const targetWaInput = document.getElementById("target-wa-input");
const addTargetBtn = document.getElementById("add-target-btn");
const targetChips = document.getElementById("target-chips");

const editAddTargetBtn = document.getElementById("edit-add-target-btn");
const editTargetInput = document.getElementById("edit-target-input");

export function renderTargetChips() {
  if (!targetChips) return;
  targetChips.innerHTML = "";
  globals.targetNumbers.forEach((num, idx) => {
    const chip = document.createElement("span");
    chip.className = "chip";
    const label =
      globals.groupsCache && globals.groupsCache[num]
        ? globals.groupsCache[num]
        : globals.contactsCache && globals.contactsCache[num]
          ? globals.contactsCache[num]
          : num;
    chip.innerHTML = `${escapeHtml(label)}<button type="button" onclick="removeTarget(${idx})">&times;</button>`;
    targetChips.appendChild(chip);
  });
}

export function renderEditTargetChips() {
  const container = document.getElementById("edit-target-chips");
  if (!container) return;
  container.innerHTML = "";
  globals.editTargetNumbers.forEach((num, idx) => {
    const chip = document.createElement("span");
    chip.className = "chip";
    const label =
      globals.groupsCache && globals.groupsCache[num]
        ? globals.groupsCache[num]
        : globals.contactsCache && globals.contactsCache[num]
          ? globals.contactsCache[num]
          : num;
    chip.innerHTML = `${escapeHtml(label)}<button type="button" onclick="removeEditTarget(${idx})">&times;</button>`;
    container.appendChild(chip);
  });
}

export function initTargetChips() {
  if (addTargetBtn)
    addTargetBtn.addEventListener("click", () => {
      const val = targetWaInput.value.trim();
      if (!val) return;

      if (!isValidWaFormat(val)) {
        showMsg(t("invalidFormat"), true);
        return;
      }

      if (globals.targetNumbers.includes(val)) {
        showMsg(t("alreadyAdded"), true);
        return;
      }
      globals.targetNumbers.push(val);
      renderTargetChips();
      targetWaInput.value = "";
      targetWaInput.focus();
    });

  if (targetWaInput)
    targetWaInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        addTargetBtn.click();
      }
    });

  window.removeTarget = (idx) => {
    globals.targetNumbers.splice(idx, 1);
    renderTargetChips();
  };

  if (editAddTargetBtn)
    editAddTargetBtn.addEventListener("click", () => {
      const val = editTargetInput.value.trim();
      if (!val) return;

      if (!isValidWaFormat(val)) {
        showMsg(t("invalidFormat"), true);
        return;
      }

      if (globals.editTargetNumbers.includes(val)) {
        showMsg(t("alreadyAdded"), true);
        return;
      }
      globals.editTargetNumbers.push(val);
      renderEditTargetChips();
      editTargetInput.value = "";
      editTargetInput.focus();
    });

  if (editTargetInput)
    editTargetInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        editAddTargetBtn.click();
      }
    });

  window.removeEditTarget = (idx) => {
    globals.editTargetNumbers.splice(idx, 1);
    renderEditTargetChips();
  };
}
