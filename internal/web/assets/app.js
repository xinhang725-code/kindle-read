/* SPDX-License-Identifier: AGPL-3.0-only */
(function () {
    'use strict';
    var state, activeBook, currentChapter, paragraphs, pages = [], pageIndex = 0, fontSize = 28;
    var view = 'shelf', busy = false, loginGeneration = 0, loginTimer, loginOTP = '', loginStopped = true;
    function el(id) { return document.getElementById(id); }
    function clear(node) { while (node.firstChild) { node.removeChild(node.firstChild); } }
    function show(name) { var list = document.querySelectorAll('.view'), i; view = name; for (i = 0; i < list.length; i++) { list[i].className = 'view' + (list[i].id === name ? '' : ' hidden'); } }
    function message(text, persistent) {
        var node = el('status'), b; clear(node); node.appendChild(document.createTextNode(text)); node.className = '';
        if (persistent) { b = document.createElement('button'); b.textContent = '关闭'; b.onclick = function () { node.className = 'hidden'; }; node.appendChild(b); }
    }
    function request(method, path, data, done) {
        var xhr = new XMLHttpRequest(); xhr.open(method, 'api/' + path, true); xhr.timeout = 30000;
        xhr.setRequestHeader('X-Kindle-Read', '1'); xhr.setRequestHeader('Content-Type', 'application/json');
        xhr.onload = function () { var value; try { value = JSON.parse(xhr.responseText); } catch (e) { done('服务响应异常，请从原生书架重新打开应用。'); return; } if (xhr.status !== 200) { done(value.error || '请求失败'); } else { done(null, value); } };
        xhr.onerror = xhr.ontimeout = function () { done('连接中断或超时。已保存的位置会保留，请重试或从原生书架重新打开。'); };
        xhr.send(data ? JSON.stringify(data) : null); return xhr;
    }
    function action(method, path, data, done) {
        if (busy) { return; } busy = true; message('正在处理…');
        request(method, path, data, function (err, value) { busy = false; el('status').className = 'hidden'; if (err) { message(err, true); return; } done(value); });
    }
    function button(parent, text, click, cls) { var b = document.createElement('button'); b.type = 'button'; b.textContent = text; b.onclick = click; if (cls) { b.className = cls; } parent.appendChild(b); return b; }
    function reload(resume) {
        action('GET', 'state', null, function (value) { state = value; document.title = value.name; el('app-name').textContent = value.name; renderShelf();
            if (resume && value.library.last) { var last = value.library.last, i; for (i = 0; i < value.library.books.length; i++) { if (value.library.books[i].id === last.book_id) { openChapter(value.library.books[i], last.chapter_id, last); return; } } }
            show('shelf');
        });
    }
    function renderShelf() {
        var bar = el('toolbar'), books = el('books'), i; clear(bar); clear(books); el('shelf-name').textContent = state.name;
        if (state.source === 'miuread') {
            el('intro').textContent = state.account.id ? state.account.name + ' · 本版本支持登录和账号书架，微信读书正文阅读仍在开发。' : '登录微信读书，查看你的账号书架。正文阅读仍在开发。';
            if (state.account.id) {
                button(bar, '刷新账号书架', syncShelf);
                button(bar, '退出账号', function () { if (window.confirm('退出账号并清理本账号在此应用中的缓存与进度？')) { action('POST', 'logout', {}, function () { reload(false); }); } });
            } else { button(bar, '扫码登录微信读书', startLogin); }
        } else {
            el('intro').textContent = '添加官方链接，尝试读取公开作品。部分网页尚未适配，账号与付费章节暂不支持。';
            button(bar, '添加作品', function () { show('add'); el('book-url').focus(); });
        }
        for (i = 0; i < state.library.books.length; i++) { (function (book) {
            var b = button(books, '', function () { var p = state.library.progress[book.id]; if (p) { openChapter(book, p.chapter_id, p); } else { catalog(book); } }, 'book');
            var title = document.createElement('span'), detail = document.createElement('span'); title.className = 'book-title'; title.textContent = book.title; detail.className = 'book-detail';
            detail.textContent = (book.local ? '本地文本 · ' : '') + (book.author || '') + (state.library.progress[book.id] ? ' · 继续阅读' : ' · ' + (book.chapters.length ? book.chapters.length + ' 章' : '正文待接入'));
            b.appendChild(title); b.appendChild(detail);
        }(state.library.books[i])); }
    }
    function catalog(book) {
        activeBook = book; show('catalog'); el('catalog-title').textContent = book.title; var list = el('chapters'), i; clear(list);
        if (!book.chapters.length) { var p = document.createElement('p'); p.textContent = '这本书已在账号书架中。微信读书目录与正文解码尚未迁移，本版本暂不能打开正文。'; list.appendChild(p); return; }
        for (i = 0; i < book.chapters.length; i++) { (function (chapter) { button(list, chapter.title + (chapter.locked ? ' · 需要授权' : ''), function () { openChapter(book, chapter.id); }, 'chapter'); }(book.chapters[i])); }
    }
    function fillPage(node, parts) { clear(node); var i, p; for (i = 0; i < parts.length; i++) { p = document.createElement('p'); p.textContent = parts[i].text; node.appendChild(p); } }
    function layout(anchor) {
        var height = Math.max(100, window.innerHeight - 76 - 44 - 42 - 72), measure = el('measure');
        el('page').style.height = height + 'px'; measure.style.height = height + 'px'; el('page').style.fontSize = fontSize + 'px'; measure.style.fontSize = fontSize + 'px';
        pages = Reader.paginate(paragraphs, function (text, parts) { fillPage(measure, parts); return measure.scrollHeight <= measure.clientHeight; });
        pageIndex = Reader.locate(pages, anchor); clear(measure); renderPage();
    }
    function renderPage() {
        if (!pages.length) { message('章节没有可显示的正文。', true); return; }
        fillPage(el('page'), pages[pageIndex].parts); el('page-number').textContent = (pageIndex + 1) + ' / ' + pages.length;
        el('chapter-title').textContent = currentChapter.title; el('previous').disabled = pageIndex === 0 && chapterIndex() === 0;
        el('next').textContent = pageIndex === pages.length - 1 ? '下一章' : '下一页';
    }
    function progress() { var a = pages[pageIndex].anchor; return {book_id: activeBook.id, chapter_id: currentChapter.id, paragraph: a.paragraph, offset: a.offset, font_size: fontSize}; }
    function save(done) {
        if (view !== 'reader' || !pages.length) { done(); return; }
        var p = progress(); action('POST', 'progress', p, function () { state.library.progress[p.book_id] = p; state.library.last = p; done(); });
    }
    function openChapter(book, id, anchor, atEnd) {
        action('GET', 'chapter?book=' + encodeURIComponent(book.id) + '&chapter=' + encodeURIComponent(id), null, function (value) {
            activeBook = book; currentChapter = value.chapter; paragraphs = value.paragraphs; fontSize = anchor && anchor.font_size ? anchor.font_size : fontSize; show('reader');
            try { layout(anchor); if (atEnd) { pageIndex = pages.length - 1; renderPage(); } save(function () {}); } catch (e) { message('无法完成排版，请减小字号或重新打开章节。', true); }
        });
    }
    function chapterIndex() { var i; for (i = 0; i < activeBook.chapters.length; i++) { if (activeBook.chapters[i].id === currentChapter.id) { return i; } } return 0; }
    function turn(delta) {
        if (busy || !pages.length) { return; }
        var next = pageIndex + delta, old = pageIndex, index;
        if (next >= 0 && next < pages.length) {
            pageIndex = next; renderPage(); var p = progress(); busy = true;
            request('POST', 'progress', p, function (err) { busy = false; if (err) { pageIndex = old; renderPage(); message('翻页位置未保存：' + err, true); } else { state.library.progress[p.book_id] = p; state.library.last = p; } });
        } else { index = chapterIndex() + delta; if (index < 0 || index >= activeBook.chapters.length) { message(delta > 0 ? '已到目录末尾。' : '已到书籍开头。', true); return; } save(function () { openChapter(activeBook, activeBook.chapters[index].id, null, delta < 0); }); }
    }
    function changeFont(delta) { if (busy || !pages.length) { return; } var anchor = pages[pageIndex].anchor; fontSize = Math.max(20, Math.min(48, fontSize + delta)); try { layout(anchor); save(function () {}); } catch (e) { message('当前字号无法排版，请减小字号。', true); } }
    function syncShelf() { action('POST', 'shelf', {}, function () { reload(false); }); }
    function pollLogin() {
        if (loginStopped) { return; }
        request('POST', 'login/poll', {generation: loginGeneration, otp: loginOTP}, function (err, result) {
            if (loginStopped) { return; }
            if (err) { loginStopped = true; message(err, true); return; }
            if (result.state === 'ready') { loginStopped = true; syncShelf(); return; }
            if (result.state === 'otp') { el('otp-form').className = ''; el('login-status').textContent = '请输入手机上显示的验证码。'; return; }
            loginTimer = window.setTimeout(pollLogin, 2000);
        });
    }
    function startLogin() {
        action('POST', 'login/start', {}, function (value) { loginGeneration = value.generation; loginOTP = ''; loginStopped = false; el('qr').src = value.url + '?generation=' + value.generation; el('otp-form').className = 'hidden'; el('login-status').textContent = '用微信扫描二维码并确认授权。'; show('login'); pollLogin(); });
    }
    function cancelLogin(done) { loginStopped = true; window.clearTimeout(loginTimer); request('POST', 'login/cancel', {}, function () { done(); }); }
    el('back').onclick = function () { if (view === 'login') { cancelLogin(function () { reload(false); }); return; } save(function () { reload(false); }); };
    el('exit').onclick = function () { save(function () { action('POST', 'exit', {}, function () { message('已保存阅读位置。可返回 Kindle 书架。', true); }); }); };
    el('local-open').onclick = function () { show('local'); };
    el('add-form').onsubmit = function (e) { e.preventDefault(); action('POST', 'book', {url: el('book-url').value}, function () { reload(false); }); };
    el('local-form').onsubmit = function (e) { e.preventDefault(); action('POST', 'import', {title: el('local-title').value, text: el('local-text').value}, function () { el('local-text').value = ''; reload(false); }); };
    el('previous').onclick = function () { turn(-1); }; el('next').onclick = function () { turn(1); }; el('font-down').onclick = function () { changeFont(-2); }; el('font-up').onclick = function () { changeFont(2); };
    el('catalog-open').onclick = function () { save(function () { catalog(activeBook); }); };
    el('login-cancel').onclick = function () { cancelLogin(function () { reload(false); }); };
    el('otp-form').onsubmit = function (e) { e.preventDefault(); loginOTP = el('otp').value; el('otp-form').className = 'hidden'; pollLogin(); };
    window.onresize = function () { if (view === 'reader' && pages.length && !busy) { var anchor = pages[pageIndex].anchor; try { layout(anchor); save(function () {}); } catch (e) { message('排版失败，请重新打开章节。', true); } } };
    // The helper expires after five minutes without requests. Hidden pages do not keep it alive.
    window.setInterval(function () { if (!document.hidden) { request('GET', 'health', null, function () {}); } }, 30000);
    document.addEventListener('visibilitychange', function () { if (document.hidden && view === 'login') { cancelLogin(function () {}); } });
    reload(true);
}());
