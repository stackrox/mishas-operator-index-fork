CATALOGS = catalog-bundle-object/rhacs-operator/catalog.json catalog-csv-metadata/rhacs-operator/catalog.json
# OPM v1.46.0 or newer is required to manipulate the files here.
OPM_VERSION = v1.48.0

MAKEFLAGS += "-j 2"

OPM = .bin/opm-$(OPM_VERSION)

.PHONY: valid-catalogs
valid-catalogs: $(CATALOGS) $(OPM)
	$(OPM) validate catalog-bundle-object
	$(OPM) validate catalog-csv-metadata

.PHONY: clean
clean:
	rm -f $(CATALOGS)
	rm -rf catalog-migrate $$(dirname $(OPM))

catalog-bundle-object/rhacs-operator/catalog.json: catalog-template.yaml $(OPM)
	mkdir -p "$$(dirname "$@")"
	$(OPM) alpha render-template basic --migrate-level none $< > $@

catalog-csv-metadata/rhacs-operator/catalog.json: catalog-template.yaml $(OPM)
	mkdir -p "$$(dirname "$@")"
	$(OPM) alpha render-template basic --migrate-level bundle-object-to-csv-metadata $< > $@

$(OPM):
	mkdir -p "$$(dirname $@)"
	os_name="$$(uname | tr '[:upper:]' '[:lower:]')"; \
	arch="$$(go env GOARCH 2>/dev/null || echo amd64)"; \
	for attempt in $$(seq 5); do \
		if curl --silent --fail --location --output $@.tmp "https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$${os_name}-$${arch}-opm"; then break; fi; \
	done
	chmod +x $@.tmp
	mv $@.tmp $@

# This is broken due to concurrency if invoked together with other targets (e.g. `make import-legacy valid-catalogs` - don't do this).
# Instead invoke `make import-legacy && make valid-catalogs`.
# TODO: fix it. Otherwise this target will disappear once konflux index builds replace the CPaaS-based ones.
.PHONY: import-legacy
import-legacy: $(OPM)
	$(OPM) migrate registry.redhat.io/redhat/redhat-operator-index:v4.12 ./catalog-migrate
	$(OPM) alpha convert-template basic --output yaml ./catalog-migrate/rhacs-operator/catalog.json > catalog-template.yaml
