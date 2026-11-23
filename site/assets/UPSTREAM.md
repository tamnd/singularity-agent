# Upstream

`site.css`, `site.js`, `icon.svg` and `fonts/` come from [math2-agent](https://github.com/tamnd/math2-agent) at commit `a82a3ddc7a828fed3258dee146c0c7249b3747d2` (`site/assets`), under Apache-2.0. The fonts keep their own licences in `fonts/`.

`site.css` has no change, so the site looks the same as Math 2.0. The style that Math 2.0 does not have is in `singularity.css`, which loads after `site.css`. Part of `singularity.css` comes from `riemann.css` of [riemann-agent](https://github.com/tamnd/riemann-agent).

`site.js` has two changed status texts of the search. To take a new version, copy the files again and apply the same two changes.
