# Bundled offline routing data

These text lists are a fixed snapshot from [MetaCubeX/meta-rules-dat](https://github.com/MetaCubeX/meta-rules-dat/tree/1fc5de325d54d3b77d6b86a1db900c5e48ae1003), revision `1fc5de325d54d3b77d6b86a1db900c5e48ae1003`, retrieved on 2026-11-06. See `sources.json` for the upstream path, size and SHA-256 of each unchanged list. Upstream copyright and GPL-3.0 terms are retained in `LICENSE.txt`.

Jeemi groups these lists into editable local AI, Telegram/Instagram and YouTube/Netflix rules. Names, descriptions and Emoji are Jeemi metadata. The client bundles these files and does not fetch updates from the repository. User edits remain local and are not replaced by creating another preset.

The general preset uses inline `GEOSITE,CN` and `GEOIP,CN` rules referencing databases managed by Jeemi's GEO feature, rather than a bundled CN domain/IP snapshot. Database installation/import remains separate. The process preset has no CN GEO rules.

`process-direct.yaml` is a Jeemi-authored, editable set of process expressions, based on the application categories in the user-provided Jeeyio Windows routing reference. It includes directory boundaries, exact process names and common macOS app bundles. The two system update domain exceptions and the general-versus-process ordering follow the reference configurations. No private subscription URL, node or credential is stored here. These local defaults are covered by the project's GPL-3.0 license.
