class POTTED {
  constructor($scope, $config) {
    if ($scope == "") {
      this.scope = $("");
      this.container = "";
    } else {
      this.scope = $($scope);
      this.container = $scope;
    }
    if ($config) {
      this.config = $config;
      this.isConfigured = true;
    } else {
      this.isConfigured = false;
      this.config = {};
    }
    this.timestamp = new Date() - 0;
    this.value = null;
  }

  // manually set the scope
  SetScope($selector) {
    this.scope = $($selector);
    this.container = $selector;
    return this;
  }

  // set rule configuration
  SetConfig($config) {
    this.config = $config;
    // TODO: check properties exists
    // TODO: check properties value not empty
    this.isConfigured = true;
    return this;
  }

  // get configuration
  GetConfig() {
    if (this.isConfigured) {
      this.config.ListContainer = this.container;
      return this.config;
    }
    return {};
  }

  // expose static utility methods
  static fn = {
    $: $,
  };

  // expose method to get DOM text
  static Text($selector) {
    return $($selector).text().trim();
  }

  // get text from selector
  GetText($selector, $parentSelector) {
    let scope = this.scope;
    if ($parentSelector != "") scope = $($parentSelector);
    return scope.find($selector).text().trim();
  }

  // expose method to get DOM link address
  static Link($selector) {
    return $($selector).attr("href").trim();
  }

  // get the link address according to the selector
  GetLink($selector, $parentSelector) {
    let scope = this.scope;
    if ($parentSelector != "") scope = $($parentSelector);
    return scope.find($selector).attr("href").trim();
  }

  // Rule fields carry selector/literal expressions; explicit custom helpers
  // GetText/GetLink retain their historical selector-only contracts.
  GetField($expression, $method, $parentSelector) {
    let scope = this.scope;
    if ($parentSelector != "") scope = $($parentSelector);
    return RSSCanReadField($expression, $method, scope);
  }

  // get data according to configuration
  GetData($config) {
    if (this.scope.length == 0) { this.value = "[]"; return this; }
    if ($config) this.SetConfig($config);
    if (this.isConfigured == false) { this.value = "[]"; return this; }

    if (this.scope.length > 1000) throw new Error("browser result exceeds item limit");
    if (!RSSCanFieldExpression(this.config.Title).value.trim() || !RSSCanFieldExpression(this.config.Link).value.trim()) throw new Error("native CSR parser requires Title and Link expressions");
    let result = [];
    let self = this;
    this.scope.each((_, el) => {
      if (self.config.Title && self.config.Link) {
        const title = self.GetField(self.config.Title, "text", el);
        let author = "";
        if (self.config.Author) {
          author = self.GetField(self.config.Author, "text", el);
        }
        let category = "";
        if (self.config.Category) {
          category = self.GetField(self.config.Category, "text", el);
        }
        let date = "";
        if (self.config.DateTime) {
          date = self.GetField(self.config.DateTime, "text", el);
        }
        let description = "";
        if (self.config.Description) {
          description = self.GetField(self.config.Description, "text", el);
        }
        const link = self.GetField(self.config.Link, "href", el);
        const hooks = {};
        for (const name of ["DateTime", "Category", "Description", "Content"]) {
          if (name !== "Content" && self.config[name]) continue;
          const hook = self.config[name + "Hook"];
          if (hook && hook.action === "readlink") hooks[name] = self.GetField(hook.URL || "", "href", el);
        }
        result.push({ title, author, category, date, description, link, __rsscan_native: true, __rsscan_hooks: hooks });
      }
    });
    this.value = JSON.stringify(result);
    return this;
  }

  PostScript() {
    // TODO: add post script hook
  }

  valueOf() {
    return this.value;
  }
}
