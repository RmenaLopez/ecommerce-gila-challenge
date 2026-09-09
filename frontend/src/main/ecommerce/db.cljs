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
   :route nil})
