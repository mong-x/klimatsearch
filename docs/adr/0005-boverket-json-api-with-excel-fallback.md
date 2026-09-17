# Boverket JSON API with Excel fallback

Boverket publishes Klimatdatabas as open data through Azure APIM. The OpenAPI server is `https://api.boverket.se/klimatdatabas`. Live GETs without a subscription key return 200 for `GetLatestVersion`, `GetAllVersions`, `GetAllResources/{latest|senaste}/{sv|en}/json`, `GetAllCategories`, and `GetResourcesByCategory?code=`. Default `BOVERKET_API_BASE` is that server URL. JSON is preferred; Excel remains the fallback when JSON fails or returns no Resources. Optional `BOVERKET_SUBSCRIPTION_KEY` is still sent as `Ocp-Apim-Subscription-Key` if set. A JSON failure must not fail boot.

Fallback is the versioned Excel pair:

- https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-klimatdatabas-version-02.07.000-sv-se.xlsx
- https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-climate-database-version-02.07.000-en-gb.xlsx

Swedish/English workbooks are merged on Resource ID. Typical A1–A3 is the "typiskt värde" / "typical value" column, not the conservative factor. Tests use recorded fixtures; the official workbooks are not committed.
