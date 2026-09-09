(ns ecommerce.db)

(def default-db
  {:products []
   :loading? false
   :error    nil
   :product-form {:submitting? false
                  :error       nil}
   ;; Cart lines keyed by product id: {"<id>" {:product {...} :quantity N}}.
   ;; No backend cart exists at all (see README) — this is purely a frontend
   ;; accumulator, flattened into one POST /orders request at checkout.
   :cart {}
   :checkout {:submitting? false
              :error       nil
              :problems    nil
              :order       nil}
   ;; Set by the :navigated event, fired on every route change (including
   ;; the very first page load) — nil only in the instant before that first
   ;; fire.
   :route nil
   :import {:submitting? false
            :error       nil
            :result      nil}
   ;; The backend defaults :limit to 20 and never returns a total count, so
   ;; "is there a next page" is inferred (see next-page/prev-page in
   ;; events.cljs), not read from the response. :q/:category are the *active*
   ;; search filters actually driving the current fetch — not what's
   ;; currently typed into the search form, which is its own local state
   ;; (see search-form in views.cljs) until submitted.
   :products-page {:limit 20 :offset 0 :q "" :category ""}})
