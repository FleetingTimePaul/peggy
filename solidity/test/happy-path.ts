import chai from "chai";
import { ethers } from "@nomiclabs/buidler";
import { solidity } from "ethereum-waffle";

import { deployContracts } from "../test-utils";
import {
  getSignerAddresses,
  makeCheckpoint,
  signHash,
  examplePowers
} from "../test-utils/pure";

chai.use(solidity);
const { expect } = chai;


describe("Peggy happy path with combination method", function () {
  it("Happy path", async function () {

    // DEPLOY CONTRACTS
    // ================

    const signers = await ethers.getSigners();
    const peggyId = ethers.utils.formatBytes32String("foo");

    const valset0 = {
      // This is the power distribution on the Cosmos hub as of 7/14/2020
      powers: examplePowers(),
      validators: signers.slice(0, examplePowers().length),
      nonce: 0
    }

    const powerThreshold = 6666;

    const {
      peggy,
      testERC20,
      checkpoint: deployCheckpoint
    } = await deployContracts(peggyId, valset0.validators, valset0.powers, powerThreshold);




    // UDPATEVALSET
    // ============

    const valset1 = (() => {
      // Make new valset by modifying some powers
      let powers = examplePowers();
      powers[0] -= 3;
      powers[1] += 3;
      let validators = signers.slice(0, powers.length);

      return {
        powers: powers,
        validators: validators,
        nonce: 1
      }
    })()

    const checkpoint1 = makeCheckpoint(
      await getSignerAddresses(valset1.validators),
      valset1.powers,
      valset1.nonce,
      peggyId
    );

    let sigs1 = await signHash(valset0.validators, checkpoint1);

    await peggy.updateValset(
      await getSignerAddresses(valset1.validators),
      valset1.powers,
      valset1.nonce,

      await getSignerAddresses(valset0.validators),
      valset0.powers,
      valset0.nonce,

      sigs1.v,
      sigs1.r,
      sigs1.s
    );

    expect(await peggy.functions.state_lastValsetCheckpoint()).to.equal(checkpoint1);




    // UDPATEVALSETANDSUBMITBATCH
    // ==========================

    const valset2 = (() => {
      // Make new valset by modifying some powers
      let powers = examplePowers();
      powers[0] -= 6;
      powers[1] += 6;
      let validators = signers.slice(0, powers.length);

      return {
        powers: powers,
        validators: validators,
        nonce: 2
      }
    })()

    const checkpoint2 = makeCheckpoint(
      await getSignerAddresses(valset2.validators),
      valset2.powers,
      valset2.nonce,
      peggyId
    );

    // Transfer out to Cosmos, locking coins
    await testERC20.functions.approve(peggy.address, 1000);
    await peggy.functions.sendToCosmos(
      testERC20.address,
      ethers.utils.formatBytes32String("myCosmosAddress"),
      1000
    );

    // Transferring into ERC20 from Cosmos
    const numTxs = 100;
    const txDestinationsInt = new Array(numTxs);
    const txFees = new Array(numTxs);
    const txAmounts = new Array(numTxs);
    for (let i = 0; i < numTxs; i++) {
      txFees[i] = 1;
      txAmounts[i] = 1;
      txDestinationsInt[i] = signers[i + 5];
    }

    const txDestinations = await getSignerAddresses(txDestinationsInt);

    const batchNonce = 1

    const methodName = ethers.utils.formatBytes32String(
      "valsetAndTransactionBatch"
    );

    let abiEncoded = ethers.utils.defaultAbiCoder.encode(
      [
        "bytes32",
        "bytes32",
        "bytes32",
        "uint256[]",
        "address[]",
        "uint256[]",
        "uint256",
        "address"
      ],
      [
        peggyId,
        methodName,
        checkpoint2,
        txAmounts,
        txDestinations,
        txFees,
        batchNonce,
        testERC20.address
      ]
    );

    let digest = ethers.utils.keccak256(abiEncoded);

    let sigs = await signHash(valset2.validators, digest);

    await peggy.updateValsetAndSubmitBatch(

      await getSignerAddresses(valset2.validators),
      valset2.powers,
      valset2.nonce,

      await getSignerAddresses(valset1.validators),
      valset1.powers,
      valset1.nonce,

      sigs.v,
      sigs.r,
      sigs.s,

      txAmounts,
      txDestinations,
      txFees,
      1,
      testERC20.address
    );

    expect(await peggy.functions.state_lastValsetCheckpoint()).to.equal(checkpoint2);

    expect(
      await (
        await testERC20.functions.balanceOf(await signers[6].getAddress())
      ).toNumber()
    ).to.equal(1);
  });
});
