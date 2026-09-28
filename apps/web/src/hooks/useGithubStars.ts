import { useEffect, useState } from "react";

const REPO = "ocultar-dev/ocultar";
const CACHE_KEY = "ocultar_gh_stars";
const CACHE_TTL_MS = 60 * 60 * 1000;

export const useGithubStars = () => {
  const [stars, setStars] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;

    const cached = (() => {
      try {
        const raw = localStorage.getItem(CACHE_KEY);
        if (!raw) return null;
        const { count, ts } = JSON.parse(raw) as { count: number; ts: number };
        if (Date.now() - ts > CACHE_TTL_MS) return null;
        return count;
      } catch {
        return null;
      }
    })();

    if (cached !== null) {
      setStars(cached);
      return;
    }

    fetch(`https://api.github.com/repos/${REPO}`)
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (cancelled || !data) return;
        const count = data.stargazers_count as number;
        setStars(count);
        try {
          localStorage.setItem(CACHE_KEY, JSON.stringify({ count, ts: Date.now() }));
        } catch {
          // storage unavailable (private mode, quota) — non-fatal, badge just won't cache
        }
      })
      .catch(() => {
        // network/rate-limit failure — badge stays hidden, nav still works
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return stars;
};
