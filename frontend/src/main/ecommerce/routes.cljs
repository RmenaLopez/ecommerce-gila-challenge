(ns ecommerce.routes)

;; Each entry is [url-pattern route-data]. :name is what code elsewhere
;; refers to the route by (subs, hrefs, the :navigate! effect) — reitit
;; doesn't infer it from the URL, it has to be given explicitly here.
(def routes
  [["/products" {:name :products}]
   ["/products/new" {:name :products/new}]
   ["/products/:id/edit" {:name :products/edit}]
   ["/cart" {:name :cart}]])
