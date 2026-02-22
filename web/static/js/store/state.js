export const state = {
  wa_number: localStorage.getItem("rm_wa_number") || null,
  currentCursor: null,
  cursorStack: [],
  totalReminders: 0,
  pageSize: 20,
  sortBy: "",
  sortOrder: "asc",
  searchTerm: "",
  lastETag: null,
  remindersData: [],
};

export const globals = {
  activeController: null,
  targetNumbers: [],
  editTargetNumbers: [],
  groupsCache: null,
  contactsCache: null,
  messageEditors: {},
  messageCount: 1,
  editQuill: null,
  deleteId: null,
  deleteMode: "single",
};
