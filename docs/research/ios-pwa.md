# Installed web app behaviour on iOS

Research for issue #8. Question: how do installed (Home Screen) web apps behave on current iOS, and what does that mean for Spendulum's login, caching and updates?

Researched November 2026. The current release is iOS 27 / Safari 27 (27.0 to 27.2 have shipped). Unless a section says otherwise, the behaviour described here has held from iOS 17 through iOS 27. The Safari 27.0, 27.1 and 27.2 feature posts contain nothing that changes Home Screen web app cookies, storage or installation.

Spendulum context: React + TypeScript + Vite PWA (vite-plugin-pwa), Go API on the same hostname under `/api`, server-side session in an `HttpOnly; Secure; SameSite=Lax` cookie with a 30-day lifetime, no offline mode in v1, and a service worker that caches the app shell.

## Short answer

- An installed web app has its **own cookie jar and storage**, separate from Safari. Since iOS 17.2, Safari's cookies for the site are **copied once** when the user taps Add to Home Screen. After that the two never sync.
- A **persistent** (`Max-Age`/`Expires`) server-set `HttpOnly` cookie from the same origin has no WebKit lifetime cap, so the 30-day session should survive app restarts and reboots. Session cookies (no expiry) are not reliable in Home Screen apps.
- The **7-day script-storage cap does not apply** to Home Screen web apps. They are also the best candidates for `navigator.storage.persist()`. Storage can still be evicted under disk pressure.
- **Service workers are fully supported**, and each installed app has its own registration and caches. Do not cache `/api` responses.
- **Updates** arrive through the normal service worker update flow. There is no app store step and no browser reload button, so the app must detect and apply updates itself.
- **Standalone quirks**: no browser chrome, no reload, and no pull-to-refresh. Safe areas need `viewport-fit=cover` plus `env()`. Inputs with text under 16px zoom the page on focus.

## 1. Cookies: shared with Safari or separate?

**Separate, with a one-time copy at install.**

- Apple's WWDC23 session says: "Home Screen web apps have a standalone, app-like experience on iOS, with separate cookies and storage from the browser." [WWDC23 "What's new in web apps"](https://developer.apple.com/videos/play/wwdc2023/10120/)
- From iOS 17.2 / iPadOS 17.2, adding to the Home Screen copies cookies: "when they tap "Add to Home Screen" and create a web app, the cookies are copied over, including the information about login state." This works from Safari, Safari View Controller and other browsers. [WebKit Features in Safari 17.2](https://webkit.org/blog/14787/)
- Only cookies are copied: "This will only work if the authentication state is stored within cookies. No other kind of local storage is copied over." After that, "no other website data is shared." [Safari 17.2](https://webkit.org/blog/14787/)
- The same session explains the reverse direction for Mac web apps, and the iOS model is the same: if the user logs in to the site in the browser after installing, they "will not be logged into the web app that has already been added ... since cookies and storage are separate after the web app is added." [WWDC23](https://developer.apple.com/videos/play/wwdc2023/10120/)
- Tracking Prevention docs confirm that the Home Screen web app's data is kept apart from Safari's. [Tracking Prevention in WebKit](https://webkit.org/tracking-prevention/), section "Home Screen Web Application Domain Exempt From ITP"

## 2. Does a 30-day HttpOnly session cookie survive restarts and reboots?

**Yes for a persistent cookie. Not reliably for a session cookie.**

- WebKit's lifetime caps target cookies created by JavaScript (7 days, or 24 hours on link-decorated landing pages) and cookies set by **third-party** CNAME- or IP-cloaked responses (7 days). Its table lists first-party, non-cloaked responses as "No cap." [Tracking Prevention in WebKit](https://webkit.org/tracking-prevention/)
- WebKit's own guidance for first-party login is a "server-set Secure and HttpOnly cookie." [Full Third-Party Cookie Blocking and More (2020)](https://webkit.org/blog/10218/full-third-party-cookie-blocking-and-more/)
- The IP-cloaking cap applies only when the response's IP address looks third-party compared with the main site. For IPv4 that means a shared prefix shorter than /16. [Private Browsing 2.0](https://webkit.org/blog/15697/private-browsing-2-0/), [WebKit bug 246477](https://bugs.webkit.org/show_bug.cgi?id=246477). Spendulum serves the API on the same hostname, so this does not apply.
- Session cookies (no `Max-Age`/`Expires`) are unreliable in Home Screen apps. [WebKit bug 272325](https://bugs.webkit.org/show_bug.cgi?id=272325) ("REGRESSION (iOS 17.x): Session cookies being reset randomly in a Home Screen web app") is still NEW. A WebKit engineer reproduced it on iOS 17.5, and a commenter reported it on iOS 18.1. Even without that bug, closing a standalone app ends the browsing session, so session cookies are not expected to survive.
- No primary source states a WebKit maximum cookie lifetime that would cut 30 days short.

**Inference** (not stated verbatim by Apple): a `Set-Cookie` with `Max-Age=2592000; HttpOnly; Secure; SameSite=Lax` from the same origin persists across app kills and device reboots for the full 30 days. It is lost only if the user deletes the web app, clears website data, or the server expires it.

## 3. Storage eviction and the 7-day cap

**The 7-day cap does not apply. Eviction under pressure still can.**

- The 7-day cap on script-writable storage covers IndexedDB, LocalStorage, SessionStorage, and service worker registrations and caches. For Home Screen web apps: "Their days of use will match actual use of the web application which resets the timer," and WebKit does not expect their data to be deleted. [Full Third-Party Cookie Blocking and More](https://webkit.org/blog/10218/full-third-party-cookie-blocking-and-more/)
- "Home Screen Web Application Domain Exempt From ITP": exempt from "ITP's 7-day cap on all script-writeable storage." [Tracking Prevention in WebKit](https://webkit.org/tracking-prevention/)
- Quotas: a standalone web app "has the same origin quota and overall quota as when it is opened in a browser app." That is up to 60% of disk per origin and 80% overall. [Updates to Storage Policy](https://webkit.org/blog/14403/updates-to-storage-policy/)
- Eviction: data is best-effort by default. It can be evicted when the overall quota is exceeded, under storage pressure, or after long disuse, in least-recently-used order by origin. An origin is excluded if it has an active page or its storage is persistent. `navigator.storage.persist()` is granted on heuristics "like whether the website is opened as a Home Screen Web App." [Updates to Storage Policy](https://webkit.org/blog/14403/updates-to-storage-policy/)

## 4. Service worker support and caching of authenticated API responses

- Service workers, the Cache API and Web Push all work in Home Screen web apps. Web Push works only in Home Screen web apps, from iOS 16.4. [Web Push for Web Apps on iOS and iPadOS](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)
- A web app no longer needs a service worker or manifest to be installable. From iOS 26, any site added to the Home Screen opens as a web app by default, with an "Open as Web App" toggle the user can turn off. "including Service Workers in your code can greatly enhance the user experience." [WebKit Features in Safari 26.0](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/)
- Safari 27.0 added service worker static routing and fixed route matching and unregistration bugs. [WebKit Features for Safari 27.0](https://webkit.org/blog/18325/webkit-features-for-safari-27-0/)
- Because the installed app has its own storage (section 1), its service worker registration and Cache Storage are separate from Safari's copy of the site.
- Caching `/api` responses: nothing on iOS forbids it. The Cache API is plain origin storage, and the service worker can see cookie-authenticated responses. The risk is in Spendulum itself: cached per-user financial data would outlive logout and could be served to the next user on the device. vite-plugin-pwa's `generateSW` precaches only build assets unless `runtimeCaching` is configured. [vite-plugin-pwa generateSW](https://vite-pwa-org.netlify.app/workbox/generate-sw)
- Navigation fallback: `generateSW` serves `index.html` for navigations. Any `/api` URL the browser *navigates* to (for example a logout redirect or OAuth callback) must be excluded with `workbox.navigateFallbackDenylist: [/^\/api\//]`. Otherwise the service worker answers with the app shell. [vite-plugin-pwa generateSW](https://vite-pwa-org.netlify.app/workbox/generate-sw), [workbox-build](https://developer.chrome.com/docs/workbox/modules/workbox-build)

## 5. How updates reach an installed web app

- There is no store step. The installed app loads from the site, and any new build reaches users through the service worker update flow. Nothing in iOS release notes describes a separate update mechanism for Home Screen apps.
- vite-plugin-pwa offers two modes:
  - `registerType: 'prompt'` (the default). `onNeedRefresh` fires when a new service worker is waiting. Calling `updateSW()` activates it and reloads. [Prompt for new content](https://vite-pwa-org.netlify.app/guide/prompt-for-update.html)
  - `registerType: 'autoUpdate'`. This forces `skipWaiting` and `clientsClaim`. Open pages reload only if the app imports a virtual module such as `virtual:pwa-register`. The docs warn "the user can lose data in any browser windows/tabs" and recommend `prompt` for apps with forms. They also say switching from `autoUpdate` to `prompt` later "can be a pain." [Automatic reload](https://vite-pwa-org.netlify.app/guide/auto-update.html)
- Update checks happen on navigation and registration. A standalone app is often resumed from memory with no navigation, so it can run an old build for a long time. vite-plugin-pwa documents a periodic `r.update()` in `onRegisteredSW`, checking every hour in its example. [Periodic service worker updates](https://vite-pwa-org.netlify.app/guide/periodic-sw-updates.html)
- Keep `cleanupOutdatedCaches` on (the default). [Prompt for new content](https://vite-pwa-org.netlify.app/guide/prompt-for-update.html)

## 6. Standalone-mode quirks

- **No browser chrome.** There is no address bar or reload button. From iOS 26 every Home Screen site opens this way unless the user turns off "Open as Web App." [Safari 26.0](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/)
- **Pull-to-refresh.** No primary source found. Developer reports consistently say standalone iOS web apps do not get Safari's pull-to-refresh, so apps add a reload button or their own gesture, for example [fmaclen/canutin#449](https://github.com/fmaclen/canutin/issues/449). Treat this as observed behaviour, not documented behaviour.
- **Safe areas.** Use `<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">`, then pad with `env(safe-area-inset-*)`, combined with normal margins via `max()`. [Designing Websites for iPhone X](https://webkit.org/blog/7929/designing-websites-for-iphone-x/). A status-bar bug in fullscreen web apps after iOS 26.1 ([WebKit bug 301994](https://bugs.webkit.org/show_bug.cgi?id=301994)) is REOPENED, but it only affects `display: fullscreen`. Use `standalone`.
- **Input zoom.** On focus, WebKit zooms so that text reaches 16px: `webViewStandardFontSize = 16`, `scale = webViewStandardFontSize / fontSize`. [WKWebViewIOS.mm](https://github.com/WebKit/WebKit/blob/main/Source/WebKit/UIProcess/API/ios/WKWebViewIOS.mm). Set form controls (amount and description fields) to at least `font-size: 16px` to avoid the zoom. Disabling user scaling is the wrong fix.
- **Keyboard.** No Apple document describes keyboard behaviour specific to standalone mode. Safari 27.0 fixed iOS bugs with selection in editable content, datalist typing, and page scrolling while selecting text inside fixed-position elements. [Safari 27.0](https://webkit.org/blog/18325/webkit-features-for-safari-27-0/). Plan to test sticky bottom bars with the keyboard open on a real device.
- **Fixed/rubber-band.** Safari 27.0 fixed fixed-position elements flickering during rubber-band scrolling. [Safari 27.0](https://webkit.org/blog/18325/webkit-features-for-safari-27-0/)
- **Stale UI on resume.** Resuming from the background does not reload the page. Refetch data on `visibilitychange`. This follows from standard page lifecycle behaviour. Developer reports describe stale Home Screen apps after background and foreground, for example [tiann/hapi#1048](https://github.com/tiann/hapi/issues/1048).

## Implications for Spendulum

### Login

- Keep the design: the server-side session in a persistent, server-set `HttpOnly; Secure; SameSite=Lax` cookie with `Max-Age` (not a session cookie) on the same hostname. It is not subject to any WebKit lifetime cap and survives restarts and reboots.
- Never keep auth state in localStorage. Only cookies are copied at install.
- A user who logs in in Safari *before* installing is logged in automatically in the installed app (iOS 17.2+). A user who installs first, or logs in in Safari later, must log in inside the app. The login screen must work well in standalone mode.
- Logging out in Safari does not log out the installed app, and vice versa. Server-side revocation is the only way to end every session.
- When the session expires, the app must route to login by itself (on a 401 from `/api`). There is no reload button to recover.
- Consider sliding renewal: re-issue the cookie with a fresh `Max-Age` on activity, so daily users are not logged out every 30 days.

### Caching

- Precache only the app shell. Do not add `runtimeCaching` for `/api`. Have the Go API send `Cache-Control: no-store` on authenticated responses so neither the HTTP cache nor a future runtime cache keeps financial data.
- Set `workbox.navigateFallbackDenylist: [/^\/api\//]`.
- Storage eviction is not a correctness risk for v1, because nothing important lives client-side. If offline mode comes later, call `navigator.storage.persist()`, which is likely granted for Home Screen apps.

### Updates

- Use `registerType: 'prompt'` (the default). The app has forms, which is the docs' own recommendation. Decide before the first production deploy.
- Add a periodic update check (`onRegisteredSW` plus `r.update()`) and also check on `visibilitychange` to visible. Installed apps are resumed, not reloaded.
- Show a small "New version, reload" banner wired to `updateSW()`. Keep `cleanupOutdatedCaches` enabled.
- Version the API carefully. An old client can run for days against a new API.

### Standalone UI

- Set `display: "standalone"` in the manifest, use `viewport-fit=cover`, and pad headers and bottom bars with `env(safe-area-inset-*)`.
- Use 16px or larger text on all inputs, selects and textareas.
- Provide an in-app refresh affordance (button or pull gesture shown only in standalone mode), and refetch on resume.

## Open questions / not verified

- No Apple source states whether deleting the Home Screen icon deletes the app's cookies and storage. This is widely assumed but not confirmed here.
- Pull-to-refresh behaviour comes from developer reports only.
- WebKit bug 272325 (session cookie resets) has no fix recorded. It should not affect persistent cookies, but test login persistence on a real device after a reboot.
