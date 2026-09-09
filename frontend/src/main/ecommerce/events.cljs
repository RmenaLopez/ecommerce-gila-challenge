(ns ecommerce.events
  (:require [re-frame.core :as rf]
            [day8.re-frame.http-fx]
            [ajax.core :as ajax]
            [reitit.frontend.easy :as rfe]
            [ecommerce.db :as db]))

;; Handlers are named, top-level functions rather than inline anonymous ones
;; passed straight to reg-event-db/reg-event-fx — this is what makes each
;; one directly callable (and unit-testable) on its own, independent of
;; re-frame's registry. See events_test.cljs.

(defn initialize-db
  [_ _]
  db/default-db)

(rf/reg-event-db :initialize-db initialize-db)

;; Fired by core.cljs's on-navigate callback every time the route changes
;; (including the very first page load). Storing the whole match (route
;; name, path params, etc.) lets subs read whatever piece they need.
(defn navigated
  [db [_ match]]
  (assoc db :route match))

(rf/reg-event-db :navigated navigated)

;; A custom effect, the same idea as :dispatch or :http-xhrio but one we're
;; registering ourselves instead of getting from re-frame or a library —
;; reg-fx is how you teach re-frame to understand a new effect key. Handlers
;; describe "navigate to this route" as data (:navigate! :products); this is
;; the one place that description actually gets carried out, via reitit's
;; own push-state function, keeping handlers themselves free of direct,
;; imperative calls to anything.
(rf/reg-fx
 :navigate!
 (fn [route-name]
   (rfe/push-state route-name)))

;; api-base is hardcoded to the local dev backend for now — this project has
;; no build-time environment config yet, and the frontend/backend only ever
;; run on these two fixed local ports during development.
(def api-base "http://localhost:8080")

(defn fetch-products
  [{:keys [db]} _]
  {:db         (assoc db :loading? true :error nil)
   :http-xhrio {:method          :get
                :uri             (str api-base "/products")
                :response-format (ajax/json-response-format {:keywords? true})
                :on-success      [:products-loaded]
                :on-failure      [:products-load-failed]}})

(rf/reg-event-fx :fetch-products fetch-products)

(defn products-loaded
  [db [_ products]]
  (assoc db :products products :loading? false))

(rf/reg-event-db :products-loaded products-loaded)

(defn products-load-failed
  [db [_ _error]]
  (assoc db :error "Failed to load products." :loading? false))

(rf/reg-event-db :products-load-failed products-load-failed)


(defn create-product
  [{:keys [db]} [_ product]]
  {:db         (-> db
                   (assoc-in [:product-form :submitting?] true)
                   (assoc-in [:product-form :error] nil))
   :http-xhrio {:method          :post
                :uri             (str api-base "/products")
                :params          product
                :format          (ajax/json-request-format)
                :response-format (ajax/json-response-format {:keywords? true})
                :on-success      [:product-created]
                :on-failure      [:create-product-failed]}})

(rf/reg-event-fx :create-product create-product)

(defn product-created
  [{:keys [db]} [_]]
  {:db (-> db
           (assoc-in [:product-form :submitting?] false)
           (assoc-in [:product-form :error] nil))
   :dispatch  [:fetch-products]
   :navigate! :products})

(rf/reg-event-fx :product-created product-created)

(defn create-product-failed
  [db [_ error]]
  (-> db
      (assoc-in [:product-form :submitting?] false)
      (assoc-in [:product-form :error]
                (get-in error [:response :error] "Failed to create product."))))

(rf/reg-event-db :create-product-failed create-product-failed)

(defn update-product
  [{:keys [db]} [_ id product]]
  {:db         (-> db
                   (assoc-in [:product-form :submitting?] true)
                   (assoc-in [:product-form :error] nil))
   :http-xhrio {:method          :put
                :uri             (str api-base "/products/" id)
                :params          product
                :format          (ajax/json-request-format)
                :response-format (ajax/json-response-format {:keywords? true})
                :on-success      [:product-updated]
                :on-failure      [:update-product-failed]}})

(rf/reg-event-fx :update-product update-product)

(defn product-updated
  [{:keys [db]} [_]]
  {:db        (-> db
                  (assoc-in [:product-form :submitting?] false)
                  (assoc-in [:product-form :error] nil))
   :dispatch  [:fetch-products]
   :navigate! :products})

(rf/reg-event-fx :product-updated product-updated)

(defn update-product-failed
  [db [_ error]]
  (-> db
      (assoc-in [:product-form :submitting?] false)
      (assoc-in [:product-form :error]
                (get-in error [:response :error] "Failed to update product."))))

(rf/reg-event-db :update-product-failed update-product-failed)

;; --- Cart (frontend-only accumulator, no backend cart exists — see README) ---

;; (max 1 quantity) guards against a cleared/invalid quantity input still
;; adding a nonsensical zero-or-negative line.
(defn add-to-cart
  [db [_ product quantity]]
  (let [quantity (max 1 quantity)]
    (update-in db [:cart (:id product)]
               (fn [existing]
                 {:product  product
                  :quantity (+ quantity (or (:quantity existing) 0))}))))

(rf/reg-event-db :add-to-cart add-to-cart)

(defn remove-from-cart
  [db [_ product-id]]
  (update db :cart dissoc product-id))

(rf/reg-event-db :remove-from-cart remove-from-cart)

(defn set-cart-quantity
  [db [_ product-id quantity]]
  (if (pos? quantity)
    (assoc-in db [:cart product-id :quantity] quantity)
    (update db :cart dissoc product-id)))

(rf/reg-event-db :set-cart-quantity set-cart-quantity)

;; --- Checkout ---

;; POST /orders wants {:items [{:product_id ... :quantity ...} ...]}, not the
;; {product-id {:product ... :quantity ...}} shape the cart is stored in —
;; mapv is map's vector-returning cousin (map itself returns a lazy seq;
;; mapv is what you want when you need a real, realized vector back).
(defn- cart->items [cart]
  (mapv (fn [[product-id line]]
          {:product_id product-id
           :quantity   (:quantity line)})
        cart))

(defn checkout
  [{:keys [db]} _]
  {:db         (-> db
                   (assoc-in [:checkout :submitting?] true)
                   (assoc-in [:checkout :error] nil)
                   (assoc-in [:checkout :problems] nil)
                   (assoc-in [:checkout :order] nil))
   :http-xhrio {:method          :post
                :uri             (str api-base "/orders")
                :params          {:items (cart->items (:cart db))}
                :format          (ajax/json-request-format)
                :response-format (ajax/json-response-format {:keywords? true})
                :on-success      [:order-placed]
                :on-failure      [:checkout-failed]}})

(rf/reg-event-fx :checkout checkout)

;; A successful purchase decrements real stock on the backend, so
;; :fetch-products refreshes the list to reflect it — same reasoning as
;; product-created/product-updated already refetching after their own
;; successful writes.
(defn order-placed
  [{:keys [db]} [_ order]]
  {:db       (-> db
                 (assoc-in [:checkout :submitting?] false)
                 (assoc-in [:checkout :order] order)
                 (assoc :cart {}))
   :dispatch [:fetch-products]})

(rf/reg-event-fx :order-placed order-placed)

;; A rejected purchase's response shape is richer than the generic
;; {"error": "..."} used everywhere else — it also carries a :problems array
;; naming exactly which items failed and why (see order_handler.go's 409
;; response). Both are pulled out here, each with its own fallback.
(defn checkout-failed
  [db [_ error]]
  (-> db
      (assoc-in [:checkout :submitting?] false)
      (assoc-in [:checkout :error] (get-in error [:response :error] "Failed to place order."))
      (assoc-in [:checkout :problems] (get-in error [:response :problems] []))))

(rf/reg-event-db :checkout-failed checkout-failed)
