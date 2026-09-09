(ns ecommerce.subs
  (:require [re-frame.core :as rf]))

(rf/reg-sub
 :products
 (fn [db _]
   (:products db)))

(rf/reg-sub
 :loading?
 (fn [db _]
   (:loading? db)))

(rf/reg-sub
 :error
 (fn [db _]
   (:error db)))

(rf/reg-sub
 :product-form/submitting?
 (fn [db _]
   (get-in db [:product-form :submitting?])))

(rf/reg-sub
 :product-form/error
 (fn [db _]
   (get-in db [:product-form :error])))

(rf/reg-sub
 :cart
 (fn [db _]
   (:cart db)))

(rf/reg-sub
 :route
 (fn [db _]
   (:route db)))

(rf/reg-sub
 :checkout/submitting?
 (fn [db _]
   (get-in db [:checkout :submitting?])))

(rf/reg-sub
 :checkout/error
 (fn [db _]
   (get-in db [:checkout :error])))

(rf/reg-sub
 :checkout/problems
 (fn [db _]
   (get-in db [:checkout :problems])))

(rf/reg-sub
 :checkout/order
 (fn [db _]
   (get-in db [:checkout :order])))
