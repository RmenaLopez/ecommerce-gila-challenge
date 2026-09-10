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

;; Always passes the current page's limit/offset explicitly — omitting them
;; entirely is what originally hid this gap: the backend silently defaults
;; to limit=20 with no total count in the response, so a product created
;; past the first page just never showed up in the list at all.
;; js/encodeURIComponent escapes q/category for safe use inside a URL —
;; needed now that they're free-text search input rather than plain numbers
;; like limit/offset; a category value like "Home & Office" (a real one in
;; the sample CSV) would otherwise corrupt the query string's structure.
(defn fetch-products
  [{:keys [db]} _]
  (let [{:keys [limit offset q category]} (:products-page db)]
    {:db         (assoc db :loading? true :error nil)
     :http-xhrio {:method          :get
                  :uri             (str api-base "/products"
                                        "?limit=" limit
                                        "&offset=" offset
                                        "&q=" (js/encodeURIComponent q)
                                        "&category=" (js/encodeURIComponent category))
                  :response-format (ajax/json-response-format {:keywords? true})
                  :on-success      [:products-loaded]
                  :on-failure      [:products-load-failed]}}))

(rf/reg-event-fx :fetch-products fetch-products)

;; The backend never reports a total count, so "is there a next page" is
;; inferred in the view (see products-page in views.cljs) by comparing how
;; many products actually came back against the page size, not tracked here.
(defn next-page
  [{:keys [db]} _]
  {:db       (update-in db [:products-page :offset] + (get-in db [:products-page :limit]))
   :dispatch [:fetch-products]})

(rf/reg-event-fx :next-page next-page)

(defn prev-page
  [{:keys [db]} _]
  {:db       (update-in db [:products-page :offset]
                         (fn [offset] (max 0 (- offset (get-in db [:products-page :limit])))))
   :dispatch [:fetch-products]})

(rf/reg-event-fx :prev-page prev-page)

;; Submit-triggered, not live-search-as-you-type — see README's decisions
;; section for why (mostly a time-constraint call, not a strong opinion that
;; this is the only right answer). Resets :offset back to 0: staying on
;; whatever page you were browsing before would otherwise land you
;; mid-way through a completely different result set.
(defn search-products
  [{:keys [db]} [_ q category]]
  {:db       (-> db
                 (assoc-in [:products-page :q] q)
                 (assoc-in [:products-page :category] category)
                 (assoc-in [:products-page :offset] 0))
   :dispatch [:fetch-products]})

(rf/reg-event-fx :search-products search-products)

(defn clear-search
  [{:keys [db]} _]
  {:db       (-> db
                 (assoc-in [:products-page :q] "")
                 (assoc-in [:products-page :category] "")
                 (assoc-in [:products-page :offset] 0))
   :dispatch [:fetch-products]})

(rf/reg-event-fx :clear-search clear-search)

;; A separate, explicit action from name/category search — GET
;; /products/sku/{sku} returns exactly one exact match (or a 404), not a
;; filtered page, so it doesn't touch :products-page at all.
(defn lookup-by-sku
  [{:keys [db]} [_ sku]]
  {:db         (-> db
                   (assoc-in [:sku-lookup :submitting?] true)
                   (assoc-in [:sku-lookup :error] nil)
                   (assoc-in [:sku-lookup :product] nil))
   :http-xhrio {:method          :get
                :uri             (str api-base "/products/sku/" (js/encodeURIComponent sku))
                :response-format (ajax/json-response-format {:keywords? true})
                :on-success      [:sku-found]
                :on-failure      [:sku-lookup-failed]}})

(rf/reg-event-fx :lookup-by-sku lookup-by-sku)

(defn sku-found
  [db [_ product]]
  (-> db
      (assoc-in [:sku-lookup :submitting?] false)
      (assoc-in [:sku-lookup :product] product)))

(rf/reg-event-db :sku-found sku-found)

(defn sku-lookup-failed
  [db [_ error]]
  (-> db
      (assoc-in [:sku-lookup :submitting?] false)
      (assoc-in [:sku-lookup :error] (get-in error [:response :error] "No product found with that SKU."))))

(rf/reg-event-db :sku-lookup-failed sku-lookup-failed)

(defn clear-sku-lookup
  [db _]
  (-> db
      (assoc-in [:sku-lookup :product] nil)
      (assoc-in [:sku-lookup :error] nil)))

(rf/reg-event-db :clear-sku-lookup clear-sku-lookup)

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

;; --- CSV import ---

;; A CSV upload isn't JSON, so it can't use :params + :format like every
;; other request here — the backend's handler reads a multipart file field
;; (r.FormFile("file")), which means the browser needs to send a real
;; multipart/form-data body. js/FormData is the browser API for building
;; exactly that; handing it to :body (instead of :params) tells http-xhrio
;; "send this body as-is" — the browser then sets the correct
;; multipart/form-data Content-Type (with its required boundary) on its own,
;; the same way it would for a plain HTML <form> file upload.
;; mode is an extra plain field alongside the file itself — the backend
;; reads it via r.FormValue("mode") (see import_handler.go), same FormData
;; object, just a second .append instead of a file.
(defn- file->form-data [file mode]
  (doto (js/FormData.)
    (.append "file" file)
    (.append "mode" mode)))

(defn import-csv
  [{:keys [db]} [_ file mode]]
  {:db         (-> db
                   (assoc-in [:import :submitting?] true)
                   (assoc-in [:import :error] nil)
                   (assoc-in [:import :result] nil))
   :http-xhrio {:method          :post
                :uri             (str api-base "/products/import")
                :body            (file->form-data file mode)
                :response-format (ajax/json-response-format {:keywords? true})
                :on-success      [:csv-imported]
                :on-failure      [:csv-import-failed]}})

(rf/reg-event-fx :import-csv import-csv)

;; A successful import writes real products, so :fetch-products refreshes
;; the list — same reasoning as every other successful write in this file.
(defn csv-imported
  [{:keys [db]} [_ result]]
  {:db       (-> db
                 (assoc-in [:import :submitting?] false)
                 (assoc-in [:import :result] result))
   :dispatch [:fetch-products]})

(rf/reg-event-fx :csv-imported csv-imported)

(defn csv-import-failed
  [db [_ error]]
  (-> db
      (assoc-in [:import :submitting?] false)
      (assoc-in [:import :error] (get-in error [:response :error] "Import failed."))))

(rf/reg-event-db :csv-import-failed csv-import-failed)

;; --- Delete product ---

;; DELETE /products/{id} responds 204 No Content on success — no JSON body
;; to parse, so text-response-format (which trivially accepts an empty
;; string) is used instead of json-response-format here.
(defn delete-product
  [_ [_ id]]
  {:http-xhrio {:method          :delete
                :uri             (str api-base "/products/" id)
                ;; No :params/:body here, but cljs-ajax still requires an
                ;; explicit :format for any non-GET request (unlike GET,
                ;; which never even tries to write a body) — omitting it
                ;; throws "unrecognized request format" even with nothing
                ;; to actually send.
                :format          (ajax/text-request-format)
                :response-format (ajax/text-response-format)
                :on-success      [:product-deleted]
                :on-failure      [:delete-product-failed]}})

(rf/reg-event-fx :delete-product delete-product)

(defn product-deleted
  [_ _]
  {:dispatch [:fetch-products]})

(rf/reg-event-fx :product-deleted product-deleted)

;; text-response-format means a failure's :response is a plain string, not a
;; parsed map, so there's no specific backend message to pull out here —
;; just a generic fallback, reusing the same top-level :error the product
;; list already shows.
(defn delete-product-failed
  [db _]
  (assoc db :error "Failed to delete product."))

(rf/reg-event-db :delete-product-failed delete-product-failed)
