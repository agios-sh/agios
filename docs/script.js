document.querySelectorAll(".copy").forEach((btn) => {
  btn.addEventListener("click", () => {
    const block = btn.closest(".command-block");
    const text = block.dataset.copy;

    navigator.clipboard.writeText(text).then(
      () => {
        btn.textContent = "copied";
        btn.dataset.copied = "true";

        setTimeout(() => {
          btn.textContent = "copy";
          delete btn.dataset.copied;
        }, 1200);
      },
      () => {
        btn.textContent = "error";

        setTimeout(() => {
          btn.textContent = "copy";
        }, 1200);
      }
    );
  });
});
