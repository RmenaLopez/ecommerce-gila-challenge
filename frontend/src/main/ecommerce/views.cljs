(ns ecommerce.views
  (:require [reagent.core :as r]
            [re-frame.core :as rf]
            [reitit.frontend.easy :as rfe]))

;; --- Small parsing/formatting helpers shared by product-row, the cart, and
;;     the create/edit form ---

(defn- cents->dollar-string [cents]
  (.toFixed (/ cents 100) 2))

(defn format-price [cents]
  (str "$" (cents->dollar-string cents)))

;; Dollars are what the user types (e.g. "89.99"); the backend wants integer
;; cents (see README's "Product JSON responses expose price_cents directly"
;; decision). Math/round guards against float noise from (* 89.99 100) —
;; without it, some inputs land on e.g. 8998.999999999999 instead of 8999.
(defn- dollars->cents [s]
  (let [n (js/parseFloat s)]
    (if (js/isNaN n) 0 (Math/round (* n 100)))))

(defn- parse-int [s]
  (let [n (js/parseInt s 10)]
    (if (js/isNaN n) 0 n)))

(defn- parse-float [s]
  (let [n (js/parseFloat s)]
    (if (js/isNaN n) 0 n)))

;; Reagent "form-2" component: the outer function runs once per mount,
;; creating `qty` fresh for this specific row. Local, not app-db — nothing
;; else needs to know what quantity is currently typed into one row's input.
(defn product-row [product]
  (let [qty (r/atom 1)]
    (fn [product]
      [:tr
       [:td (:name product)]
       [:td (:sku product)]
       [:td (format-price (:price_cents product))]
       [:td (:stock product)]
       [:td
        [:input {:type      "number"
                 :min       1
                 :value     @qty
                 :on-change (fn [e] (reset! qty (parse-int (-> e .-target .-value))))}]
        [:button {:on-click #(rf/dispatch [:add-to-cart product @qty])} "Add to cart"]]
       [:td [:a {:href (rfe/href :products/edit {:id (:id product)})} "Edit"]]])))

(defn product-list []
  (let [products @(rf/subscribe [:products])
        loading? @(rf/subscribe [:loading?])
        error    @(rf/subscribe [:error])]
    [:div
     (cond
       loading? [:p "Loading products..."]
       error    [:p {:style {:color "red"}} error]
       :else
       [:table
        [:thead
         [:tr [:th "Name"] [:th "SKU"] [:th "Price"] [:th "Stock"] [:th "Add to cart"] [:th]]]
        [:tbody
         (for [product products]
           ^{:key (:id product)} [product-row product])]])]))

;; --- Cart (frontend-only accumulator, no backend cart exists — see README) ---

(defn- cart-line [product-id line]
  (let [product  (:product line)
        quantity (:quantity line)]
    [:tr
     [:td (:name product)]
     [:td (format-price (:price_cents product))]
     [:td
      [:button {:on-click #(rf/dispatch [:set-cart-quantity product-id (dec quantity)])} "-"]
      quantity
      [:button {:on-click #(rf/dispatch [:set-cart-quantity product-id (inc quantity)])} "+"]]
     [:td (format-price (* (:price_cents product) quantity))]
     [:td [:button {:on-click #(rf/dispatch [:remove-from-cart product-id])} "Remove"]]]))

(defn- cart-total [cart]
  (reduce + 0 (map (fn [line] (* (:price_cents (:product line)) (:quantity line)))
                    (vals cart))))

(defn cart-view []
  (let [cart        @(rf/subscribe [:cart])
        submitting? @(rf/subscribe [:checkout/submitting?])
        error       @(rf/subscribe [:checkout/error])
        problems    @(rf/subscribe [:checkout/problems])
        order       @(rf/subscribe [:checkout/order])]
    [:div
     [:h2 "Cart"]
     (when order
       [:p {:style {:color "green"}}
        "Order placed! ID: " (:id order) " — Total: " (format-price (:total_cents order))])
     (when error
       [:div {:style {:color "red"}}
        [:p error]
        (when (seq problems)
          [:ul (for [problem problems]
                 ^{:key (:product_id problem)}
                 [:li (:product_id problem) ": " (:reason problem)])])])
     (if (empty? cart)
       [:p "Cart is empty."]
       [:div
        [:table
         [:thead [:tr [:th "Product"] [:th "Price"] [:th "Quantity"] [:th "Subtotal"] [:th]]]
         [:tbody
          (for [[product-id line] cart]
            ^{:key product-id} [cart-line product-id line])]]
        [:p "Total: " (format-price (cart-total cart))]
        [:button {:on-click #(rf/dispatch [:checkout]) :disabled submitting?}
         (if submitting? "Placing order..." "Checkout")]])]))

;; --- Create/edit product form ---

(def ^:private empty-fields
  {:sku "" :name "" :description "" :category "" :price "" :stock "" :weight-kg ""})

(defn- build-product-payload [fields]
  {:sku         (:sku fields)
   :name        (:name fields)
   :description (:description fields)
   :category    (:category fields)
   :price_cents (dollars->cents (:price fields))
   :stock       (parse-int (:stock fields))
   :weight_kg   (parse-float (:weight-kg fields))})

;; The reverse of build-product-payload: takes a real product (as returned by
;; the backend) and turns it back into the string-keyed shape the form's
;; local atom uses, so editing pre-fills every field with the current values.
(defn- product->fields [product]
  {:sku         (:sku product)
   :name        (:name product)
   :description (:description product)
   :category    (:category product)
   :price       (cents->dollar-string (:price_cents product))
   :stock       (str (:stock product))
   :weight-kg   (str (:weight_kg product))})

;; e.-target.-value reads a DOM input's current text (`.-` means "read this
;; property," as opposed to `.foo` which calls a method — see
;; knowledge/clojurescript-syntax-basics.md for the full explanation).
(defn- text-field [label field-key fields disabled?]
  [:div
   [:label label]
   [:input {:type      "text"
            :value     (get @fields field-key)
            :disabled  disabled?
            :on-change (fn [e]
                         (swap! fields assoc field-key (-> e .-target .-value)))}]])

;; Reagent "form-2" component: this outer function runs once per mount,
;; creating `fields` fresh — pre-filled from editing-product when editing, or
;; empty when creating. Now that create/edit each live on their own route
;; (/products/new, /products/:id/edit), navigating between them always
;; mounts a fresh instance of whichever page is showing, which is what
;; resets fields correctly — the same effect the old :key-based remount
;; achieved manually before routing existed.
(defn product-form [editing-product]
  (let [fields (r/atom (if editing-product
                          (product->fields editing-product)
                          empty-fields))]
    (fn [editing-product]
      (let [submitting? @(rf/subscribe [:product-form/submitting?])
            error       @(rf/subscribe [:product-form/error])
            editing?    (some? editing-product)]
        [:form
         {:on-submit (fn [e]
                       (.preventDefault e)
                       (if editing?
                         (rf/dispatch [:update-product (:id editing-product) (build-product-payload @fields)])
                         (rf/dispatch [:create-product (build-product-payload @fields)])))}
         (when error [:p {:style {:color "red"}} error])
         [text-field "SKU" :sku fields editing?]
         [text-field "Name" :name fields false]
         [text-field "Description" :description fields false]
         [text-field "Category" :category fields false]
         [text-field "Price ($)" :price fields false]
         [text-field "Stock" :stock fields false]
         [text-field "Weight (kg)" :weight-kg fields false]
         [:button {:type "submit" :disabled submitting?}
          (cond
            submitting? "Saving..."
            editing?    "Save changes"
            :else       "Save product")]
         " "
         [:a {:href (rfe/href :products)} "Cancel"]]))))

;; --- Pages (one per route) ---

(defn products-page []
  [:div
   [:a {:href (rfe/href :products/new)} "Add product"]
   [product-list]])

(defn product-new-page []
  [product-form nil])

;; Looks the product up from the already-loaded :products list rather than
;; fetching it individually — simple, and correct in practice since init
;; always fetches the list before routing can even happen. Known gap: a
;; fresh page load landing directly on this route before that fetch
;; resolves shows "Loading..." and then renders correctly once :products
;; arrives (this component re-renders reactively either way), but an
;; outright invalid id would show "Loading..." forever rather than a real
;; not-found message.
(defn product-edit-page []
  (let [route    @(rf/subscribe [:route])
        id       (get-in route [:path-params :id])
        products @(rf/subscribe [:products])
        product  (first (filter #(= (:id %) id) products))]
    (if product
      [product-form product]
      [:p "Loading..."])))

(defn nav []
  [:nav
   [:a {:href (rfe/href :products)} "Products"]
   " | "
   [:a {:href (rfe/href :cart)} "Cart"]])

;; case matches route-name against each literal option in turn — like cond,
;; but comparing one value against a fixed set of possibilities instead of
;; evaluating a separate condition per branch. The final, un-paired form
;; ([:p "Not found."]) is case's default, used when nothing else matches.
(defn current-page []
  (let [route      @(rf/subscribe [:route])
        route-name (get-in route [:data :name])]
    (case route-name
      ;; nil covers the bare root URL (no # fragment at all) — nothing
      ;; matches any route pattern there, so route-name is nil rather than
      ;; :products; treating it the same as :products makes the product
      ;; list the effective home page instead of a dead-end "not found."
      (:products nil) [products-page]
      :products/new   [product-new-page]
      :products/edit  ^{:key (get-in route [:path-params :id])} [product-edit-page]
      :cart           [cart-view]
      [:p "Not found."])))

(defn app []
  [:div
   [:h1 "E-Commerce"]
   [nav]
   [current-page]])
