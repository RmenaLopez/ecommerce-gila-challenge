(ns ecommerce.core
  (:require [reagent.dom :as rdom]))

(defn app []
  [:div
   [:h1 "E-Commerce"]
   [:p "Frontend scaffold is up."]])

(defn mount-root []
  (rdom/render [app] (.getElementById js/document "app")))

(defn init []
  (mount-root))

(defn reload []
  (mount-root))
