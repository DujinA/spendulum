# iPhone checklist

A manual check on a real iPhone, for the human operator. Agents never run it. Playwright cannot check these behaviours of the home-screen web app.

## When to run

- After a deploy that touches the web app manifest, the service worker or PWA config (`web/src/pwa.ts`), `backend/internal/auth/` or the session cookie, or the update prompt.
- Once before the app goes into daily use.

## Steps

1. In Safari, open the app and add it to the Home Screen.
2. Open it from the Home Screen and log in. The home-screen web app has its own cookie jar, separate from Safari.
3. Close the app from the app switcher and open it again: you are still logged in.
4. Restart the iPhone and open the app: you are still logged in.
5. Deploy a new version, then bring the app back from the background: the update prompt appears, and **Reload** loads the new version.
6. Make the session expire (or delete it on the server), then use the app: a `401` takes you to the login screen.
7. Check the screen edges: nothing sits under the notch, the Dynamic Island or the home indicator, in portrait and landscape.
8. Tap into every kind of input: the page does not zoom in on focus.
9. Switch the iPhone between light and dark mode: the app follows.
