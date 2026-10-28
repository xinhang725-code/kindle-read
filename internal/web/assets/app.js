/* SPDX-License-Identifier: AGPL-3.0-only */
(function () {
    'use strict';
    var state, activeBook, currentChapter, paragraphs, blocks = [], pages = [], pageIndex = 0, fontSize = 28, logicalAnchor = null;
    var loginRequired = false, toolsVisible = false, shelfPage = 0, catalogPage = 0, searchPage = 1, rankings = [], rankIndex = 0, browseMode = 'rank';
    var spacing = 1.75, margin = 26, bold = false, indent = true;
    var view = 'shelf', busy = false, loginGeneration = 0, loginAttempt = 0, loginTimer, loginOTP = '', loginStopped = true;
    function el(id) { return document.getElementById(id); }
    function clear(node) { while (node.firstChild) { node.removeChild(node.firstChild); } }
    function isCP() { return state && state.source === 'changpei'; }
    function bodyClass() { document.body.className = (isCP() ? 'changpei' : '') + (view === 'reader' ? ' reading' : '') + (isCP() && !toolsVisible ? ' tools-hidden' : ''); }
    function show(name) { if (isCP() && loginRequired && name !== 'login') { return; } var list = document.querySelectorAll('.view'), i; view = name; for (i = 0; i < list.length; i++) { list[i].className = 'view' + (list[i].id === name ? '' : ' hidden'); } bodyClass(); window.scrollTo(0, 0); }
    function loginScreen(automatic) {
        loginRequired = true; activeBook = null; currentChapter = null; pages = []; paragraphs = []; blocks = []; clear(el('page')); clear(el('books')); clear(el('results')); clear(el('chapters'));
        el('login-title').textContent = '长佩阅读'; el('login-instructions').textContent = '打开长佩阅读 App → 我的 → 扫一扫'; el('login-help').textContent = '请在手机上确认登录后开始阅读。';
        el('qr').className = 'hidden'; el('login-cancel').className = 'hidden'; el('back').disabled = true; show('login');
        if (automatic) { startLogin(); } else { el('login-status').textContent = '请刷新二维码，重新扫码登录。'; }
    }
    function message(text, persistent) {
        var node = el('status'), b; clear(node); node.appendChild(document.createTextNode(text)); node.className = '';
        if (persistent) { b = document.createElement('button'); b.textContent = '关闭'; b.onclick = function () { node.className = 'hidden'; }; node.appendChild(b); }
    }
    function request(method, path, data, done) {
        var xhr = new XMLHttpRequest(); xhr.open(method, 'api/' + path, true); xhr.timeout = 30000;
        xhr.setRequestHeader('X-Kindle-Read', '1'); xhr.setRequestHeader('Content-Type', 'application/json');
        xhr.onload = function () { var value; try { value = JSON.parse(xhr.responseText); } catch (e) { done('服务响应异常，请从原生书架重新打开应用。'); return; } if (xhr.status !== 200) { done(value.error || value.message || '请求失败', value); } else { done(null, value); } };
        xhr.onerror = xhr.ontimeout = function () { done('连接中断或超时。已保存的位置会保留，请重试或从原生书架重新打开。'); };
        xhr.send(data ? JSON.stringify(data) : null); return xhr;
    }
    function action(method, path, data, done) {
        if (busy) { return; } busy = true; message('正在处理…');
        request(method, path, data, function (err, value) { busy = false; el('status').className = 'hidden'; if (err) { if (isCP() && value && (value.code === 'AUTH_REQUIRED' || value.code === 'AUTH_EXPIRED')) { cancelLogin(function () {}); loginScreen(false); } message(err, true); if (path === 'state') { button(el('status'), '重新连接', function () { reload(true); }); } return; } done(value); });
    }
    function button(parent, text, click, cls) { var b = document.createElement('button'); b.type = 'button'; b.textContent = text; b.onclick = click; if (cls) { b.className = cls; } parent.appendChild(b); return b; }
    function hasChapter(book, id) { var i; for (i = 0; i < book.chapters.length; i++) { if (book.chapters[i].id === id) { return true; } } return false; }
    function reload(resume) {
        action('GET', 'state', null, function (value) { state = value; document.title = value.name; el('app-name').textContent = value.name;
            if (isCP()) { fontSize = 20; el('local-open').className = 'hidden'; if (value.login_required) { loginScreen(true); return; } }
            loginRequired = false; el('back').disabled = false; renderShelf();
            if (resume && value.library.last) { var last = value.library.last, i; for (i = 0; i < value.library.books.length; i++) { if (value.library.books[i].id === last.book_id && hasChapter(value.library.books[i], last.chapter_id)) { openChapter(value.library.books[i], last.chapter_id, last); return; } } }
            show('shelf');
        });
    }
    function pager(node, page, total, change) { clear(node); if (total < 2) { return; } var previous = button(node, '上一页', function () { change(page - 1); }); previous.disabled = page === 0; var text = document.createElement('span'); text.textContent = (page + 1) + ' / ' + total; node.appendChild(text); var next = button(node, '下一页', function () { change(page + 1); }); next.disabled = page + 1 >= total; }
    function logout() { if (window.confirm('退出账号并清理本账号在此应用中的缓存与进度？')) { action('POST', 'logout', {}, function () { shelfPage = 0; reload(false); }); } }
    function renderShelf() {
        var bar = el('toolbar'), books = el('books'), i, perPage = window.innerWidth < 600 ? 4 : 8, start; clear(bar); clear(books); el('shelf-name').textContent = state.name;
        if (isCP()) {
            el('intro').textContent = (state.account.name || '长佩读者') + ' · 此设备的阅读书架';
            button(bar, '逛书城', discover); button(bar, '搜索作品', searchOpen); button(bar, '添加作品', function () { show('add'); }); button(bar, '退出账号', logout);
            if (state.library.last) { button(bar, '继续阅读', function () { reload(true); }, 'selected'); }
        } else if (state.source === 'miuread') {
            el('intro').textContent = state.account.id ? state.account.name + ' · 本版本支持登录和账号书架，微信读书正文阅读仍在开发。' : '登录微信读书，查看你的账号书架。正文阅读仍在开发。';
            if (state.account.id) {
                button(bar, '刷新账号书架', syncShelf);
                button(bar, '退出账号', function () { if (window.confirm('退出账号并清理本账号在此应用中的缓存与进度？')) { action('POST', 'logout', {}, function () { reload(false); }); } });
            } else { button(bar, '扫码登录微信读书', startLogin); }
        } else {
            el('intro').textContent = '添加官方链接，尝试读取公开作品。部分网页尚未适配，账号与付费章节暂不支持。';
            button(bar, '添加作品', function () { show('add'); });
        }
        shelfPage = Math.min(shelfPage, Math.max(0, Math.ceil(state.library.books.length / perPage) - 1)); start = isCP() ? shelfPage * perPage : 0;
        for (i = start; i < state.library.books.length && (!isCP() || i < start + perPage); i++) { (function (book) {
            var resume = state.library.progress[book.id]; if (resume && !hasChapter(book, resume.chapter_id)) { resume = null; }
            var b = button(books, '', function () { if (resume) { openChapter(book, resume.chapter_id, resume); } else { catalog(book); } }, 'book' + (resume ? ' resume' : ''));
            var title = document.createElement('span'), detail = document.createElement('span'); title.className = 'book-title'; title.textContent = book.title; detail.className = 'book-detail';
            detail.textContent = (book.local ? '本地文本 · ' : '') + (book.author || '') + (resume ? ' · 继续阅读' : ' · ' + (book.chapters.length ? book.chapters.length + ' 章' : '正文待接入'));
            b.appendChild(title); b.appendChild(detail);
        }(state.library.books[i])); }
        pager(el('shelf-pages'), shelfPage, isCP() ? Math.ceil(state.library.books.length / perPage) : 0, function (p) { shelfPage = p; renderShelf(); });
    }
    function rememberBook(book) { var i; for (i = 0; i < state.library.books.length; i++) { if (state.library.books[i].id === book.id) { state.library.books[i] = book; return; } } state.library.books.push(book); }
    function addBook(id) { action('POST', 'book', {url: id}, function (book) { rememberBook(book); catalogPage = 0; catalog(book); }); }
    function results(books) { var list = el('results'), i; clear(list); if (!books.length) { list.textContent = '暂时没有找到作品。'; } for (i = 0; i < books.length; i++) { (function (book) { var b = button(list, '', function () { addBook(book.id); }, 'book'), title = document.createElement('span'), detail = document.createElement('span'); title.className = 'book-title'; title.textContent = book.title; detail.className = 'book-detail'; detail.textContent = book.author + ' · 加入书架并打开目录'; b.appendChild(title); b.appendChild(detail); }(books[i])); } }
    function renderRank() { var bar = el('rank-tabs'), i, rank = rankings[rankIndex]; clear(bar); clear(el('search-pages')); if (!rank) { results([]); return; } for (i = 0; i < rankings.length; i++) { (function (idx) { button(bar, rankings[idx].title, function () { rankIndex = idx; searchPage = 1; renderRank(); }, idx === rankIndex ? 'selected' : ''); }(i)); } var size = Math.max(3, Math.min(5, Math.floor((window.innerHeight - 350) / 112))); results(rank.books.slice((searchPage - 1) * size, searchPage * size)); pager(el('search-pages'), searchPage - 1, Math.ceil(rank.books.length / size), function (p) { searchPage = p + 1; renderRank(); }); }
    function discover() { browseMode = 'rank'; show('browse'); el('browse-title').textContent = '书城'; el('search-form').className = 'hidden'; if (rankings.length) { renderRank(); return; } action('POST', 'discover', {}, function (value) { rankings = value.rankings; rankIndex = 0; searchPage = 1; renderRank(); }); }
    function searchOpen() { browseMode = 'search'; searchPage = 1; show('browse'); el('browse-title').textContent = '搜索作品'; el('search-form').className = ''; clear(el('rank-tabs')); clear(el('results')); clear(el('search-pages')); }
    function searchBooks() { action('POST', 'search', {query: el('search-query').value, page: searchPage}, function (value) { results(value.books); pager(el('search-pages'), searchPage - 1, Math.ceil(value.count / 10), function (p) { searchPage = p + 1; searchBooks(); }); }); }
    function catalog(book) {
        if (!activeBook || activeBook.id !== book.id) { catalogPage = 0; } activeBook = book; show('catalog'); el('catalog-title').textContent = book.title;
        var list = el('chapters'), bar = el('catalog-actions'), i, perPage = Math.max(4, Math.min(12, Math.floor((window.innerHeight - 350) / 64))), start, saved = state.library.progress[book.id]; clear(list); clear(bar);
        if (isCP()) { button(bar, '刷新目录', function () { action('POST', 'book', {url: book.id}, function (value) { rememberBook(value); catalog(value); }); }); }
        if (!book.chapters.length) { var p = document.createElement('p'); p.textContent = isCP() ? '这本作品暂时没有可用章节。' : '这本书已在账号书架中。微信读书目录与正文解码尚未迁移，本版本暂不能打开正文。'; list.appendChild(p); clear(el('catalog-pages')); return; }
        catalogPage = Math.min(catalogPage, Math.max(0, Math.ceil(book.chapters.length / perPage) - 1)); start = isCP() ? catalogPage * perPage : 0;
        for (i = start; i < book.chapters.length && (!isCP() || i < start + perPage); i++) { (function (chapter) { button(list, chapter.title + (chapter.locked ? ' · 需要授权' : ''), function () { openChapter(book, chapter.id); }, 'chapter' + (saved && saved.chapter_id === chapter.id ? ' current-chapter' : '')); }(book.chapters[i])); }
        pager(el('catalog-pages'), catalogPage, isCP() ? Math.ceil(book.chapters.length / perPage) : 0, function (p) { catalogPage = p; catalog(book); });
    }
    function fillPage(node, parts) { clear(node); var i, p, cls; for (i = 0; i < parts.length; i++) { p = document.createElement('p'); p.textContent = parts[i].text; cls = parts[i].offset > 0 ? 'continued' : ''; if (blocks[parts[i].paragraph] && blocks[parts[i].paragraph].t === 'note') { cls += ' author-note'; } p.className = cls; if (isCP() && !indent) { p.style.textIndent = '0'; } node.appendChild(p); } }
    function layout(anchor) {
        var height = isCP() ? Math.max(100, window.innerHeight - 48) : Math.max(100, window.innerHeight - 76 - 44 - 60 - 72), measure = el('measure');
        if (isCP()) { el('main').style.paddingLeft = margin + 'px'; el('main').style.paddingRight = margin + 'px'; }
        el('page').style.height = height + 'px'; measure.style.height = height + 'px'; el('page').style.fontSize = fontSize + 'px'; measure.style.fontSize = fontSize + 'px';
        el('page').style.lineHeight = spacing; measure.style.lineHeight = spacing; el('page').style.fontWeight = bold ? 'bold' : 'normal'; measure.style.fontWeight = bold ? 'bold' : 'normal';
        pages = Reader.paginate(paragraphs, function (text, parts) { fillPage(measure, parts); return measure.scrollHeight <= measure.clientHeight; });
        pageIndex = Reader.locate(pages, anchor); logicalAnchor = anchor ? {paragraph: anchor.paragraph, offset: anchor.offset} : (pages.length ? pages[pageIndex].anchor : null); clear(measure); renderPage();
    }
    function renderPage() {
        if (!pages.length) { message('章节没有可显示的正文。', true); return; }
        fillPage(el('page'), pages[pageIndex].parts); el('page-number').textContent = (pageIndex + 1) + ' / ' + pages.length;
        el('chapter-title').textContent = currentChapter.title; el('previous').disabled = pageIndex === 0 && chapterIndex() === 0;
        el('next').textContent = pageIndex === pages.length - 1 ? '下一章' : '下一页';
        el('reading-position').textContent = currentChapter.title + ' · ' + (pageIndex + 1) + ' / ' + pages.length; el('reading-progress').style.width = ((pageIndex + 1) * 100 / pages.length) + '%';
    }
    function progress() { var a = logicalAnchor; return {book_id: activeBook.id, chapter_id: currentChapter.id, paragraph: a.paragraph, offset: a.offset, font_size: fontSize}; }
    function save(done) {
        if (view !== 'reader' || !pages.length) { done(); return; }
        var p = progress(); action('POST', 'progress', p, function () { state.library.progress[p.book_id] = p; state.library.last = p; done(); });
    }
    function openChapter(book, id, anchor, atEnd) {
        action('GET', 'chapter?book=' + encodeURIComponent(book.id) + '&chapter=' + encodeURIComponent(id), null, function (value) {
            activeBook = book; currentChapter = value.chapter; paragraphs = value.paragraphs; blocks = value.blocks || []; toolsVisible = false; el('reader-settings').className = 'hidden'; fontSize = anchor && anchor.font_size ? anchor.font_size : fontSize; show('reader');
            try { layout(anchor); if (atEnd) { pageIndex = pages.length - 1; logicalAnchor = pages[pageIndex].anchor; renderPage(); } save(function () {}); } catch (e) { message('无法完成排版，请减小字号或重新打开章节。', true); }
        });
    }
    function chapterIndex() { var i; for (i = 0; i < activeBook.chapters.length; i++) { if (activeBook.chapters[i].id === currentChapter.id) { return i; } } return 0; }
    function turn(delta) {
        if (busy || !pages.length) { return; }
        var next = pageIndex + delta, old = pageIndex, oldAnchor = logicalAnchor, index;
        if (next >= 0 && next < pages.length) {
            pageIndex = next; logicalAnchor = pages[pageIndex].anchor; renderPage(); var p = progress(); busy = true;
            request('POST', 'progress', p, function (err) { busy = false; if (err) { pageIndex = old; logicalAnchor = oldAnchor; renderPage(); message('翻页位置未保存：' + err, true); } else { state.library.progress[p.book_id] = p; state.library.last = p; } });
        } else { index = chapterIndex() + delta; if (index < 0 || index >= activeBook.chapters.length) { message(delta > 0 ? '已到目录末尾。' : '已到书籍开头。', true); return; } save(function () { openChapter(activeBook, activeBook.chapters[index].id, null, delta < 0); }); }
    }
    function changeFont(delta) { if (busy || !pages.length) { return; } var anchor = logicalAnchor; fontSize = Math.max(isCP() ? 18 : 20, Math.min(isCP() ? 26 : 48, fontSize + delta)); try { layout(anchor); save(function () {}); } catch (e) { message('当前字号无法排版，请减小字号。', true); } }
    function syncShelf() { action('POST', 'shelf', {}, function () { reload(false); }); }
    function pollLogin() {
        if (loginStopped) { return; }
        var attempt = loginAttempt, generation = loginGeneration;
        request('POST', 'login/poll', {generation: generation, otp: loginOTP}, function (err, result) {
            if (loginStopped || attempt !== loginAttempt || generation !== loginGeneration) { return; }
            if (err) { loginStopped = true; el('qr').className = 'hidden'; el('login-status').textContent = err; return; }
            if (result.state === 'ready') { loginStopped = true; if (isCP()) { loginRequired = false; reload(false); } else { syncShelf(); } return; }
            if (result.state === 'scanned') { el('login-status').textContent = '扫码成功，请在手机上确认。'; }
            if (result.state === 'otp') { el('otp-form').className = ''; el('login-status').textContent = '请输入手机上显示的验证码。'; return; }
            loginTimer = window.setTimeout(function () { if (attempt === loginAttempt) { pollLogin(); } }, 2000);
        });
    }
    function startLogin() {
        if (busy) { return; } var attempt = ++loginAttempt; loginStopped = true; window.clearTimeout(loginTimer); el('qr').className = 'hidden'; el('login-status').textContent = '正在获取二维码…';
        if (!isCP()) { el('login-title').textContent = '登录微信读书'; el('login-instructions').textContent = '用微信扫描二维码并确认授权。'; el('login-cancel').className = ''; }
        show('login');
        action('POST', 'login/start', {}, function (value) { if (attempt !== loginAttempt) { return; } loginGeneration = value.generation; loginOTP = ''; loginStopped = false; el('qr').src = value.url + '?generation=' + value.generation; el('qr').className = ''; el('otp-form').className = 'hidden'; el('login-status').textContent = isCP() ? '等待手机扫描…' : '用微信扫描二维码并确认授权。'; pollLogin(); });
    }
    function cancelLogin(done) { var attempt = ++loginAttempt; loginStopped = true; window.clearTimeout(loginTimer); request('POST', 'login/cancel', {}, function () { if (attempt === loginAttempt) { done(); } }); }
    el('back').onclick = function () { if (isCP() && loginRequired) { return; } if (view === 'login') { cancelLogin(function () { reload(false); }); return; } save(function () { reload(false); }); };
    el('exit').onclick = function () { function exit() { save(function () { action('POST', 'exit', {}, function () { message('可返回 Kindle 书架。', true); }); }); } if (view === 'login') { cancelLogin(exit); } else { exit(); } };
    el('local-open').onclick = function () { show('local'); };
    el('add-form').onsubmit = function (e) { e.preventDefault(); action('POST', 'book', {url: el('book-url').value}, function (book) { rememberBook(book); catalog(book); }); };
    el('local-form').onsubmit = function (e) { e.preventDefault(); action('POST', 'import', {title: el('local-title').value, text: el('local-text').value}, function () { el('local-text').value = ''; reload(false); }); };
    el('previous').onclick = function () { turn(-1); }; el('next').onclick = function () { turn(1); }; el('font-down').onclick = function () { changeFont(-2); }; el('font-up').onclick = function () { changeFont(2); };
    el('catalog-open').onclick = function () { save(function () { catalog(activeBook); }); };
    el('login-cancel').onclick = function () { cancelLogin(function () { reload(false); }); };
    el('otp-form').onsubmit = function (e) { e.preventDefault(); loginOTP = el('otp').value; el('otp-form').className = 'hidden'; pollLogin(); };
    el('login-refresh').onclick = startLogin;
    el('search-form').onsubmit = function (e) { e.preventDefault(); searchPage = 1; searchBooks(); };
    el('reader-return').onclick = function () { save(function () { reload(false); }); };
    function toggleTools() { if (busy || !isCP()) { return; } toolsVisible = !toolsVisible; el('reader-settings').className = 'hidden'; bodyClass(); }
    el('page').onclick = function (e) { if (!isCP() || view !== 'reader') { return; } var box = el('page').getBoundingClientRect(), x = e.clientX - box.left; if (x < box.width / 3) { turn(-1); } else if (x > box.width * 2 / 3) { turn(1); } else { toggleTools(); } };
    el('page').onkeydown = function (e) { if (e.keyCode === 37) { e.preventDefault(); turn(-1); } else if (e.keyCode === 39) { e.preventDefault(); turn(1); } else if (e.keyCode === 13 || e.keyCode === 32) { e.preventDefault(); toggleTools(); } };
    function saveSettings() { try { window.localStorage.setItem('changpei-layout', JSON.stringify({spacing:spacing,margin:margin,bold:bold,indent:indent})); } catch (ignore) {} }
    function applySettings() { if (busy || !pages.length) { return; } try { layout(logicalAnchor); saveSettings(); save(function () {}); } catch (e) { message('当前版式无法排版，请重新选择。', true); } }
    function settingButtons() { var i, values = [1.6, 1.75, 1.9], margins = [20,26,32]; clear(el('spacing-controls')); clear(el('margin-controls')); for (i = 0; i < values.length; i++) { (function (v) { button(el('spacing-controls'), '行距 ' + v, function () { if (busy) { return; } spacing = v; applySettings(); settingButtons(); }, spacing === v ? 'selected' : ''); }(values[i])); } for (i = 0; i < margins.length; i++) { (function (v) { button(el('margin-controls'), '边距 ' + v, function () { if (busy) { return; } margin = v; applySettings(); settingButtons(); }, margin === v ? 'selected' : ''); }(margins[i])); } el('bold-toggle').textContent = '加粗：' + (bold ? '开' : '关'); el('indent-toggle').textContent = '首行缩进：' + (indent ? '开' : '关'); }
    el('settings-open').onclick = function () { settingButtons(); el('reader-settings').className = ''; };
    el('settings-close').onclick = function () { el('reader-settings').className = 'hidden'; toolsVisible = false; bodyClass(); };
    el('bold-toggle').onclick = function () { if (busy) { return; } bold = !bold; applySettings(); settingButtons(); };
    el('indent-toggle').onclick = function () { if (busy) { return; } indent = !indent; applySettings(); settingButtons(); };
    try { var prefs = JSON.parse(window.localStorage.getItem('changpei-layout')); if (prefs) { if ([1.6,1.75,1.9].indexOf(prefs.spacing) >= 0) { spacing = prefs.spacing; } if ([20,26,32].indexOf(prefs.margin) >= 0) { margin = prefs.margin; } bold = prefs.bold === true; indent = prefs.indent !== false; } } catch (ignore) {}
    window.onresize = function () { if (view === 'reader' && pages.length && !busy) { var anchor = logicalAnchor; try { layout(anchor); save(function () {}); } catch (e) { message('排版失败，请重新打开章节。', true); } } };
    // The helper expires after five minutes without requests. Hidden pages do not keep it alive.
    window.setInterval(function () { if (!document.hidden) { request('GET', 'health', null, function () {}); } }, 30000);
    document.addEventListener('visibilitychange', function () { if (document.hidden && view === 'login') { cancelLogin(function () { el('qr').className = 'hidden'; el('login-status').textContent = '已暂停登录，请刷新二维码。'; }); } });
    reload(true);
}());
