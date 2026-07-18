const input = document.querySelector("#reference-filter");
const rows = [...document.querySelectorAll("tr[data-search]")];
const sections = [...document.querySelectorAll(".reference-section")];
const empty = document.querySelector("#no-results");

input?.addEventListener("input", () => {
  const query = input.value.trim().toLocaleLowerCase();
  let visibleCount = 0;

  for (const row of rows) {
    const visible = !query || row.dataset.search.includes(query);
    row.hidden = !visible;
    visibleCount += Number(visible);
  }

  for (const section of sections) {
    const searchableRows = [...section.querySelectorAll("tr[data-search]")];
    section.hidden = searchableRows.length > 0 && searchableRows.every((row) => row.hidden);
  }

  empty.hidden = visibleCount > 0;
});
