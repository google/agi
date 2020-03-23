document.addEventListener("DOMContentLoaded", function() {
  function hide() {
    var panel = document.getElementById("image-view");
    if (panel) {
      panel.classList.remove("show");
      panel.getElementsByClassName("image")[0].src = "";
    }
  }

  function getOrCreatePanel() {
    var panel = document.getElementById("image-view");
    if (panel) {
      return panel;
    }

    panel = document.createElement("div");
    panel.id = "image-view";

    var back = document.createElement("span");
    back.className = "material-icons back";
    back.innerText = "arrow_back";
    back.addEventListener("click", hide);
    panel.appendChild(back);

    var image = document.createElement("img");
    image.className = "image";
    panel.appendChild(image);

    document.body.appendChild(panel);
    document.addEventListener("keydown", function(e) {
      if (e.key == "Escape") {
        hide();
      }
    });
    return panel;
  }

  function showImage(url) {
    var panel = getOrCreatePanel();
    var image = panel.getElementsByClassName("image")[0];
    image.src = url;
    panel.classList.add("show");
  }

  function register(link) {
    if (link.href) {
      link.addEventListener("click", function(e) {
        showImage(link.href);
        e.preventDefault();
      });
    }
  }

  [].forEach.call(document.getElementsByClassName("preview"), register);
});
