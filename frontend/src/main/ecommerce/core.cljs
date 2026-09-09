(ns ecommerce.core
  (:require [reagent.dom.client :as rdom]
            [re-frame.core :as rf]
            [reitit.frontend :as rfr]
            [reitit.frontend.easy :as rfe]
            [ecommerce.routes :as routes]
            [ecommerce.events]
            [ecommerce.subs]
            [ecommerce.views :as views]))

;; `defonce` matters here: a React root must only be created once per DOM
;; element — calling create-root again (e.g. on shadow-cljs hot-reload)
;; would itself warn/error, so `root` is created a single time and reused.
(defonce root (rdom/create-root (.getElementById js/document "app")))

(defn mount-root []
  (rdom/render root [views/app]))

(def router (rfr/router routes/routes))

;; Called by reitit every time the URL's route changes — including once
;; immediately on startup, for whatever URL the page loaded with. Dispatches
;; an event (rather than reaching into app-db directly here) so this stays
;; consistent with how every other piece of state gets written: through a
;; named event, not a direct mutation from arbitrary code.
(defn- on-navigate [match]
  (rf/dispatch [:navigated match]))

(defn init []
  (rf/dispatch-sync [:initialize-db])
  (rf/dispatch [:fetch-products])
  ;; :use-fragment true means routes live after a # (e.g. /#/products/new) —
  ;; the browser never sends anything after # to the server, so loading any
  ;; route directly (a refresh, a bookmark) always just serves index.html
  ;; normally, no server-side "catch-all" route config needed.
  (rfe/start! router on-navigate {:use-fragment true})
  (mount-root))

(defn reload []
  (mount-root))
