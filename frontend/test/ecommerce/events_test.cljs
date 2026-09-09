(ns ecommerce.events-test
  (:require [cljs.test :refer [deftest is testing]]
            [ecommerce.events :as events]
            [ecommerce.db :as db]))

(deftest initialize-db-test
  (is (= db/default-db (events/initialize-db {} [:initialize-db]))))

(deftest products-loaded-test
  (testing "stores the products and clears loading"
    (let [result (events/products-loaded
                  {:loading? true :error nil :products []}
                  [:products-loaded [{:id "1" :name "Widget"}]])]
      (is (= [{:id "1" :name "Widget"}] (:products result)))
      (is (false? (:loading? result))))))

(deftest products-load-failed-test
  (testing "sets an error message and clears loading"
    (let [result (events/products-load-failed
                  {:loading? true :error nil}
                  [:products-load-failed {:some "xhrio error map"}])]
      (is (some? (:error result)))
      (is (false? (:loading? result))))))

(deftest fetch-products-test
  (testing "sets loading true, clears any previous error, and issues the right HTTP effect"
    (let [result (events/fetch-products
                  {:db {:loading? false :error "old error" :products []}}
                  [:fetch-products])]
      (is (true? (get-in result [:db :loading?])))
      (is (nil? (get-in result [:db :error])))
      (is (= :get (get-in result [:http-xhrio :method])))
      (is (= (str events/api-base "/products") (get-in result [:http-xhrio :uri])))
      (is (= [:products-loaded] (get-in result [:http-xhrio :on-success])))
      (is (= [:products-load-failed] (get-in result [:http-xhrio :on-failure]))))))
