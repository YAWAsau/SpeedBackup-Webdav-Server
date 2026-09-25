(function (root) {
  'use strict';
  const quote = value => "'" + value.replace(/'/g, "'\"'\"'") + "'";
  function config(address, username, password, anonymous=false, rootAnonymous='') {
    if ([address, username, password].some(v => /[\r\n\0]/.test(v))) throw new Error('line');
    let url;
    try { url = new URL(address); } catch { throw new Error('url'); }
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error('url');
    if (url.hostname === 'localhost' || url.hostname === '[::1]' || /^127\./.test(url.hostname) || url.hostname === '0.0.0.0' || url.hostname === '[::]') throw new Error('loopback');
    if (!/^[a-z0-9][a-z0-9_-]{0,47}$/.test(username)) throw new Error('user');
    // Canonicalize here too: copying must be correct even before input blur.
    address = shareAddress(address, username, anonymous, rootAnonymous);
    if (anonymous) return `remote_type=webdav\nwebdav_url=${quote(address)}\nwebdav_remote_user=''\nwebdav_remote_pass=''`;
    if (!password.trim()) throw new Error('password');
    return `remote_type=webdav\nwebdav_url=${quote(address)}\nwebdav_remote_user=${quote(username)}\nwebdav_remote_pass=${quote(password)}`;
  }
  function initialAddress(origin, path, info) {
    const host = new URL(origin).hostname;
    if ((host === 'localhost' || host === '[::1]' || /^127\./.test(host)) && info.selection_required) return '';
    return (host === 'localhost' || host === '[::1]' || /^127\./.test(host)) && info.preferred_url ? info.preferred_url : origin + path;
  }
  function shareAddress(address, username, anonymous, rootAnonymous='') {
    if (!/^[a-z0-9][a-z0-9_-]{0,47}$/.test(username)) return address;
    try {
      const url = new URL(address);
      const endpoint=anonymous&&username!==rootAnonymous?`/dav-public/${username}/`:'/';
      url.pathname=url.pathname==='/'?endpoint:url.pathname.replace(/\/dav(?:-public\/[a-z0-9_-]+)?\/?$/,endpoint);
      return url.toString();
    } catch { return address; }
  }
  root.SBWebDAV = Object.freeze({ config, initialAddress, shareAddress });
})(globalThis);
