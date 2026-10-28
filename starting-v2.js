/* Stock Pillow paints this window before the native display owner is acquired. */
var WRStarting = (function () {
  var token = '', timer = 0, acknowledge = 0, deadline = 0, closed = true, title = null;
  var service = 'com.isword.weread.launch';
  function finish(expected) {
    if (expected && expected !== token) return;
    if (closed) return;
    closed = true; clearTimeout(timer); clearTimeout(acknowledge);
    nativeBridge.dismissMe();
  }
  function value(name) { return nativeBridge.getStringLipcProperty(service, name); }
  function poll() {
    if (closed) return;
    try {
      var current = value('token');
      if (current !== token && /^[0-9a-f]{32}$/.test(current)) { begin(current); return; }
      if (current !== token || value('phase') !== token + ':waiting' || Date.now() >= deadline) { finish(); return; }
    } catch (error) { finish(); return; }
    timer = setTimeout(poll, 100);
  }
  function begin(request) {
    try {
      var current = value('token');
      if (!/^[0-9a-f]{32}$/.test(current) || (request && request !== current)) return;
      clearTimeout(timer); clearTimeout(acknowledge);
      token = current; deadline = Date.now() + 45000; closed = false;
      nativeBridge.setWindowSize(screen.width, screen.height);
      if (typeof WindowTitle !== 'undefined') {
        if (!title) title = new WindowTitle(WINMGR.LAYER.DIALOG, WINMGR.ROLE.DIALOG);
        title.withChanges(function () {
          this.addParam(WINMGR.KEY.WIN_IS_MODAL, WINMGR.MODALITY.MODAL);
          this.addParam(WINMGR.KEY.FLASH_ON_SHOW, WINMGR.FLASH_ON_SHOW.FORCE_FULL);
          this.addParam(WINMGR.KEY.FLASH_ON_HIDE, WINMGR.FLASH_ON_HIDE.FORCE_FULL);
          this.addParam(WINMGR.KEY.BORDER_WIDTH, 0);
          this.addParam(WINMGR.KEY.ROUNDED_CORNERS, 0);
        });
      }
      document.getElementById('message').style.fontSize = (screen.width * 30 / 1072) + 'px';
      nativeBridge.showMe();
      // Yield to the stock renderer before acknowledging the visible window.
      function ready() {
        if (closed) return;
        try {
          var logo = document.getElementById('logo');
          if (!logo.complete) { acknowledge = setTimeout(ready, 100); return; }
          if (!logo.naturalWidth) { finish(); return; }
          if (value('token') === token) nativeBridge.setLipcProperty(service, 'ready', token + ':ready');
        }
        catch (error) { finish(); }
      }
      acknowledge = setTimeout(ready, 100);
      timer = setTimeout(poll, 150);
      window.WRStartingActive = true;
    } catch (error) { finish(); }
  }
  function init() {
    nativeBridge.registerClientParamsCallback(function (json) {
      try { var p = JSON.parse(json); if (p.show) begin(p.token); } catch (error) { finish(); }
    });
    begin();
  }
  return { init: init, finish: finish };
}());
window.addEventListener('load', WRStarting.init, false);
